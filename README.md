# IPinfo Lite to Alloy-compatible MMDB

This rewrites IPinfo Lite's flat MMDB records into two schemas understood by
Grafana Alloy/Loki's `stage.geoip`:

- `ipinfo_city_compat.mmdb`: country and continent in GeoIP2 City layout
- `ipinfo_asn_compat.mmdb`: numeric ASN and organization in GeoLite2 ASN layout

On the Alpine NPMPlus LXC:

```sh
apk add --no-cache go
cd /opt/ipinfo/ipinfo-mmdb-converter
go mod tidy
go run . \
  -input /opt/ipinfo/ipinfo_lite.mmdb \
  -city-output /opt/ipinfo/ipinfo_city_compat.mmdb \
  -asn-output /opt/ipinfo/ipinfo_asn_compat.mmdb
chmod 0644 /opt/ipinfo/ipinfo_*_compat.mmdb
```

Validate a known address:

```sh
mmdblookup --file /opt/ipinfo/ipinfo_city_compat.mmdb --ip 8.8.8.8
mmdblookup --file /opt/ipinfo/ipinfo_asn_compat.mmdb --ip 8.8.8.8
```

Expected important paths are `country.iso_code`, `continent.code`,
`autonomous_system_number`, and `autonomous_system_organization`.

Use the outputs in Alloy:

```alloy
stage.geoip {
	db      = "/opt/ipinfo/ipinfo_city_compat.mmdb"
	db_type = "city"
	source  = "client_ip"
}

stage.geoip {
	db      = "/opt/ipinfo/ipinfo_asn_compat.mmdb"
	db_type = "asn"
	source  = "client_ip"
}
```

Keep these stages after the stage that extracts `client_ip` and before the
`stage.labels`/`stage.structured_metadata` stages.
