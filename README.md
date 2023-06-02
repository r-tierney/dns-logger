# DNS Traffic Capture and Storage

This Go program captures DNS (Domain Name System) traffic from specified network interfaces and stores the captured data in a ClickHouse database. It utilises the `github.com/google/gopacket` library for packet capture and processing and the `gorm.io/gorm` library for interacting with the ClickHouse database.

## Features

- Captures DNS packets from specified network interfaces.
- Extracts relevant information from the captured packets, including the timestamp, domain name, query type, and source IP address.
- Stores the captured DNS queries in a ClickHouse database.
- Stores both the raw domain queried along with a normalised version of the domain.TLD
- Supports batching of queries to improve performance and reduce database operations.
- Uses goroutines for concurrent packet capture and database insertion, maximising efficiency.

## Prerequisites

- Go 1.20 or later
- ClickHouse database
- Network interfaces with DNS traffic to capture

## Installation

1. Clone the repository or download the source code files.
2. Install the required dependencies by running the following command:
   ```
   go mod download
   ```

## Configuration

Before running the program, you need to configure the following settings:

- ClickHouse Database Connection: Update the `dsn` variable in the `clickhouseDB()` function with the appropriate connection details for your ClickHouse database. Make sure to set the correct username, password, host, port, and database name.

## Usage

To capture DNS traffic and store it in the ClickHouse database, follow these steps:

1. Build the program by running the following command:
   ```
   go build dns-logger.go
   ```
2. Set the `DNS_TRAFFIC_DB_PASS` environment variable to the clickhouse database password
3. Run the executable file:
   ```
   ./dns-logger
   ```
4. Alternatively use the provided systemd unit file to run this program as a service just ensure you set the `DNS_TRAFFIC_DB_PASS='clickhouse_db_pass'` in `/etc/default/dns-logger`

The program will start capturing DNS packets from the specified network interfaces ( eth1, eth2 by default ) and store the captured data in the ClickHouse database.

## Contributing

Contributions are welcome! If you find any issues or have suggestions for improvement, please feel free to open an issue or submit a merge request.

## License

This project is licensed under the [GPLv3 Licence](LICENCE).

## Acknowledgments

This program uses the following open-source libraries:

- [github.com/google/gopacket](https://github.com/google/gopacket)
- [gorm.io/gorm](https://gorm.io/gorm)
- [https://github.com/hashicorp/golang-lru](https://github.com/hashicorp/golang-lru)
- [https://pkg.go.dev/golang.org/x/net/publicsuffix](https://pkg.go.dev/golang.org/x/net/publicsuffix)

## Author

[Ryan Tierney](https://github.com/r-tierney)

## References

- [ClickHouse Documentation](https://clickhouse.com/docs/en/)
- [GoPacket Documentation](https://pkg.go.dev/github.com/google/gopacket)
- [GORM Documentation](https://gorm.io/docs/)
- [golang-lru Documentation](https://github.com/hashicorp/golang-lru)
- [Golang publicsuffix Documentation](https://pkg.go.dev/golang.org/x/net/publicsuffix)

