# ipinfo-mmdb-converter

Convert an IPinfo Lite MMDB database from IPinfo's flat record layout into
databases that use the conventional MaxMind GeoIP2 schemas.

The converter preserves the source network ranges and creates two output files:

- A GeoIP2 City-schema database containing country and continent data.
- A GeoLite2 ASN-schema database containing the numeric ASN and organization.

This is useful for software that can read the MMDB file format but expects
MaxMind's standard field layout rather than IPinfo's flat fields. The generated
City database contains country and continent information only; IPinfo Lite does
not provide city names or coordinates.

## Requirements

- Go 1.23 or newer
- An IPinfo Lite `.mmdb` database

## Build

```sh
git clone https://github.com/Meganitrospeed/ipinfo-mmdb-converter.git
cd ipinfo-mmdb-converter
go build -o ipinfo-mmdb-converter .
```

## Usage

```sh
./ipinfo-mmdb-converter \
  -input ./ipinfo_lite.mmdb \
  -city-output ./ipinfo_city_compat.mmdb \
  -asn-output ./ipinfo_asn_compat.mmdb
```

The program first counts the source networks and then displays an exact
conversion progress bar. Output files are written through temporary files and
renamed into place only after each database has been written successfully.

Default paths are:

```text
Input:       /opt/ipinfo/ipinfo_lite.mmdb
City output: /opt/ipinfo/ipinfo_city_compat.mmdb
ASN output:  /opt/ipinfo/ipinfo_asn_compat.mmdb
```

Run `./ipinfo-mmdb-converter -help` to list all options.

## Output schemas

The City-compatible database provides records such as:

```json
{
  "continent": {
    "code": "NA",
    "names": { "en": "North America" }
  },
  "country": {
    "iso_code": "US",
    "names": { "en": "United States" }
  }
}
```

The ASN-compatible database provides records such as:

```json
{
  "autonomous_system_number": 15169,
  "autonomous_system_organization": "Google LLC"
}
```

## Verification

With `mmdblookup` installed:

```sh
mmdblookup --file ./ipinfo_city_compat.mmdb --ip 8.8.8.8
mmdblookup --file ./ipinfo_asn_compat.mmdb --ip 8.8.8.8
```

## Grafana Alloy example

The generated files can be consumed by two `stage.geoip` stages:

```alloy
stage.geoip {
  db      = "/path/to/ipinfo_city_compat.mmdb"
  db_type = "city"
  source  = "client_ip"
}

stage.geoip {
  db      = "/path/to/ipinfo_asn_compat.mmdb"
  db_type = "asn"
  source  = "client_ip"
}
```

Place these after the stage that extracts `client_ip`.

## License

MIT
