package main

import (
    "os"
    "log"
    "net"
    "time"
    "sync"
    "strings"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
    "gorm.io/driver/clickhouse"
    "github.com/google/gopacket"
    "golang.org/x/net/publicsuffix"
    "github.com/google/gopacket/pcap"
    "github.com/google/gopacket/layers"
    "github.com/hashicorp/golang-lru/v2"
)


// Globals & settings
var (
    wg sync.WaitGroup
    dbBatchLimit int = 100_000
    lruCacheSize int = 1_000_000
    lruCache *lru.Cache[string, string]
    interfaces [2]string = [2]string{"eth1", "eth2"}
)


// Database table structure
type DnsQuery struct {
    Timestamp   time.Time `gorm:"column:timestamp;type:DateTime64(9, 'Australia/Melbourne')"`
    RawDomain   string `gorm:"column:raw_domain"`
    Domain      string `gorm:"column:domain"`
    QueryType   string `gorm:"column:query_type"`
    SrcIp       net.IP `gorm:"column:src_ip;type:IPv4"`
}


// Capture packets then send them to batchQueries to be batch inserted into Clickhouse
func capturePackets(device string, queries chan DnsQuery) {
    wg.Add(1)
    defer wg.Done()

    // Open the device for capturing ( device, snapshotLen, promisc, block until packet is received )
    handle, err := pcap.OpenLive(device, 1600, true, pcap.BlockForever)
    if err != nil { log.Fatalf("Error opening PCAP on Device: %s, Error: %s", device, err) }
    defer handle.Close()

    // Set a BPF filter to capture only DNS traffic (UDP and TCP)
    err = handle.SetBPFFilter("dst port 53")
    if err != nil { log.Fatalf("Error setting BPF filter, Error: %s", err) }

    // Start capturing packets
    packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
    for packet := range packetSource.Packets() {
        // Extract the IP layer
        ipLayer := packet.Layer(layers.LayerTypeIPv4)
        if ipLayer == nil { continue }

        // Extract the DNS layer
        dnsLayer := packet.Layer(layers.LayerTypeDNS)
        if dnsLayer == nil { continue }

        // Extract the IPv4 addresses
        ip, _ := ipLayer.(*layers.IPv4)

        // Get the DNS question from the DNS layer and send it to clickhouse
        dnsPacket := dnsLayer.(*layers.DNS)
        if len(dnsPacket.Questions) > 0 {
            dnsQuestion := dnsPacket.Questions[0]
            queries <- DnsQuery{Timestamp: time.Unix(0, time.Now().UnixNano()), RawDomain: string(dnsQuestion.Name), Domain: normaliseDomain(string(dnsQuestion.Name)), QueryType: dnsQuestion.Type.String(), SrcIp: ip.SrcIP}
        }
    }
}


// Normalise domain using public suffix & LRU Cache ( converts SubDoMain.DoMain.TLD to domain.tld )
func normaliseDomain(rawDomain string) string {
    domainWithTLD, found := lruCache.Get(rawDomain)
    if found == true {
        return domainWithTLD
    }
    domainWithTLD, err := publicsuffix.EffectiveTLDPlusOne(rawDomain)
    if err != nil {
        log.Printf("Unable to find ETLD+1 for: %s, Error: %s", rawDomain, err)
        domainWithTLD = ""
    }
    domainWithTLD = strings.ToLower(domainWithTLD)
    lruCache.Add(rawDomain, domainWithTLD)
    return domainWithTLD
}


// Append Queries to a slice until we reach the DB batch limit then insert as a batch into clickhouse
func batchQueries(queries <-chan DnsQuery, db *gorm.DB) {
    var queryCache = []DnsQuery{}
    startTime := time.Now()

    for query := range queries {
        queryCache = append(queryCache, query)

        if len(queryCache) >= dbBatchLimit || len(queryCache) > 0 && time.Since(startTime) >= time.Second {
            // Insert as a batch into clickhouse
            result := db.Create(queryCache)
            if result.Error != nil {
                log.Printf("Error inserting into DB, Error: %s", result.Error)
            }
            // Flush out the batch / cache
            queryCache = []DnsQuery{}
            // Reset the timer
            startTime = time.Now()
        }
    }
}


// Connect to clickhouse
func clickhouseDB() *gorm.DB {
    dsn := "clickhouse://db_username_here:"+os.Getenv("DNS_TRAFFIC_DB_PASS")+"@dns-traffic-ch:9000/dns_traffic?dial_timeout=10s&read_timeout=20s"
    gormLogger := logger.New(
    log.New(os.Stdout, "\r\n", log.LstdFlags), // io writer
        logger.Config{
            SlowThreshold:             time.Second,  // Slow SQL threshold
            LogLevel:                  logger.Warn,  // Log level
            IgnoreRecordNotFoundError: false,        // Include ErrRecordNotFound error for logger
            ParameterizedQueries:      false,        // Include params in the SQL log
            Colorful:                  false,        // Disable colour
        },
    )

    db, err := gorm.Open(clickhouse.New(clickhouse.Config{
        DSN: dsn,
        DefaultTableEngineOpts: "ENGINE = ReplicatedMergeTree('/clickhouse/tables/dns_traffic.dns_queries', '{replica}') PARTITION BY toYYYYMMDD(timestamp) ORDER BY (timestamp, domain) TTL toDateTime(timestamp) + toIntervalHour(6) SETTINGS index_granularity = 8192",
    }), &gorm.Config{
        Logger: gormLogger,
    })
    if err != nil {
        log.Fatalf("Failed connecting to database, Error: %s", err)
    }
    // Migrate the database schema ( Ensure the schema is the same as the DnsQuery struct )
    db.AutoMigrate(&DnsQuery{})

    return db
}


func main() {
    // Set up LRU Cache
    var err error
    lruCache, err = lru.New[string, string](lruCacheSize)
    if err != nil {
        log.Fatalf("Error setting up LRU Cache, Error: %s", err)
    }

    // Connect to clickhouse and ensure schema is same as DnsQuery struct
    db := clickhouseDB()

    // Set up channel for queries to be sent on with a buffer of 100k
    queries := make(chan DnsQuery, 100_000)
    defer close(queries)

    // Spawn the worker process waiting to receive on the query channel
    go batchQueries(queries, db)

    // For each interface capture packets on each then send to clickhouse
    for _, device := range interfaces {
        go capturePackets(device, queries)
    }

    // Ensure all child processes are finished
    time.Sleep(time.Second)
    wg.Wait()
}
