package main

import (
    "os"
    "log"
    "net"
    "sync"
    "time"
    "strings"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
    "gorm.io/driver/sqlite"
    "golang.org/x/net/publicsuffix"
    "github.com/hashicorp/golang-lru/v2"
)


// Globals & settings
var (
    wg sync.WaitGroup
    dbBatchLimit int = 2
    lruCacheSize int = 5
    lruCache *lru.Cache[string, string]
    interfaces [2]string = [2]string{"eth1", "eth2"}
)


// Database table structure
type DnsQuery struct {
    Timestamp  time.Time `gorm:"column:timestamp"`
    RawDomain  string    `gorm:"column:raw_domain"`
    Domain     string    `gorm:"column:domain"`
    QueryType  string    `gorm:"column:query_type"`
    SrcIp      net.IP    `gorm:"column:src_ip"`
}


// Capture packets then send them to batchQueries to be batch inserted into SQLite
func capturePackets(device string, queries chan DnsQuery) {
    wg.Add(1)
    defer wg.Done()

    // Pretend these are the queries we are receiving on each interface
    var domains []string
    if device == "eth1" {
        domains = []string{"google.com", "apple.com", "subdomain.testingcom.au"}
    } else {
        domains = []string{"", "youtube.com", "fAcebOok.com", "subdomain.testingcom.au"}
    }
    for _, domain := range domains {
        queries <- DnsQuery{Timestamp: time.Unix(0, time.Now().UnixNano()), RawDomain: domain, Domain: normaliseDomain(domain), QueryType: "A", SrcIp: net.ParseIP("192.168.1.2")}
    }
}


// Normalise domain using public suffix & LRU Cache ( converts SubDoMain.DoMain.TLD to domain.tld )
func normaliseDomain(rawDomain string) string {
    domainWithTLD, found := lruCache.Get(rawDomain)
    if found == true {
        log.Println("using cache for: " + domainWithTLD)
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


// Append Queries to a slice until we reach the DB batch limit then insert as a batch into SQLite
func batchQueries(queries <-chan DnsQuery, db *gorm.DB) {
    var queryCache = []DnsQuery{}
    startTime := time.Now()

    for query := range queries {
        queryCache = append(queryCache, query)

        if len(queryCache) >= dbBatchLimit || len(queryCache) > 0 && time.Since(startTime) >= time.Second {
            // Insert as a batch into SQLite
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


// Connect to SQLite
func sqliteDB() *gorm.DB {
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
    db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{
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

    // Connect to SQLite and ensure schema is same as DnsQuery struct
    db := sqliteDB()

    // Set up channel for queries to be sent on with a buffer of 100k
    queries := make(chan DnsQuery, 100_000)
    defer close(queries)

    // Spawn the worker process waiting to receive on the query channel
    go batchQueries(queries, db)

    // For each interface capture packets on each then send to SQLite
    for _, device := range interfaces {
        go capturePackets(device, queries)
    }

    // Ensure all child processes are finished
    time.Sleep(time.Second)
    wg.Wait()

    // Select / Find all rows
    var dnsQuery []DnsQuery
    db.Find(&dnsQuery)
    for _, row := range dnsQuery {
        log.Println(row)
    }
}

