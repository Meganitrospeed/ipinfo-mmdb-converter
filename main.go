package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
	"github.com/oschwald/maxminddb-golang"
)

type ipinfoRecord struct {
	ASDomain      string `maxminddb:"as_domain"`
	ASName        string `maxminddb:"as_name"`
	ASN           string `maxminddb:"asn"`
	Continent     string `maxminddb:"continent"`
	ContinentCode string `maxminddb:"continent_code"`
	Country       string `maxminddb:"country"`
	CountryCode   string `maxminddb:"country_code"`
}

func names(value string) mmdbtype.Map {
	return mmdbtype.Map{"en": mmdbtype.String(value)}
}

func main() {
	input := flag.String("input", "/opt/ipinfo/ipinfo_lite.mmdb", "source IPinfo Lite MMDB")
	cityOutput := flag.String("city-output", "/opt/ipinfo/ipinfo_city_compat.mmdb", "MaxMind City-schema output")
	asnOutput := flag.String("asn-output", "/opt/ipinfo/ipinfo_asn_compat.mmdb", "MaxMind ASN-schema output")
	flag.Parse()

	db, err := maxminddb.Open(*input)
	if err != nil {
		log.Fatalf("open source: %v", err)
	}
	defer db.Close()

	options := func(databaseType, description string) mmdbwriter.Options {
		return mmdbwriter.Options{
			DatabaseType: databaseType,
			Description:  map[string]string{"en": description},
			IPVersion:    int(db.Metadata.IPVersion),
			Languages:    []string{"en"},
			RecordSize:   28,
		}
	}

	city, err := mmdbwriter.New(options("GeoIP2-City", "IPinfo Lite converted to MaxMind City schema"))
	if err != nil {
		log.Fatalf("create city tree: %v", err)
	}
	asn, err := mmdbwriter.New(options("GeoLite2-ASN", "IPinfo Lite converted to MaxMind ASN schema"))
	if err != nil {
		log.Fatalf("create ASN tree: %v", err)
	}

	fmt.Println("Counting source networks...")
	total, err := countNetworks(db)
	if err != nil {
		log.Fatalf("count source networks: %v", err)
	}
	if total == 0 {
		log.Fatal("source database contains no networks")
	}
	fmt.Printf("Found %d networks. Converting...\n", total)

	var networks, cityRecords, asnRecords uint64
	progress := newProgressBar(total)
	iterator := db.Networks(maxminddb.SkipAliasedNetworks)
	for iterator.Next() {
		networks++
		var source ipinfoRecord
		network, err := iterator.Network(&source)
		if err != nil {
			log.Fatalf("decode network: %v", err)
		}

		cityRecord := mmdbtype.Map{}
		if source.ContinentCode != "" || source.Continent != "" {
			continent := mmdbtype.Map{}
			if source.ContinentCode != "" {
				continent["code"] = mmdbtype.String(source.ContinentCode)
			}
			if source.Continent != "" {
				continent["names"] = names(source.Continent)
			}
			cityRecord["continent"] = continent
		}
		if source.CountryCode != "" || source.Country != "" {
			country := mmdbtype.Map{}
			if source.CountryCode != "" {
				country["iso_code"] = mmdbtype.String(source.CountryCode)
			}
			if source.Country != "" {
				country["names"] = names(source.Country)
			}
			cityRecord["country"] = country
			cityRecord["registered_country"] = country
		}
		if len(cityRecord) != 0 {
			if err := city.Insert(network, cityRecord); err != nil {
				log.Fatalf("insert city %s: %v", network, err)
			}
			cityRecords++
		}

		asnNumber := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(source.ASN)), "AS")
		parsedASN, parseErr := strconv.ParseUint(asnNumber, 10, 32)
		asnRecord := mmdbtype.Map{}
		if parseErr == nil && parsedASN != 0 {
			asnRecord["autonomous_system_number"] = mmdbtype.Uint32(parsedASN)
		}
		if source.ASName != "" {
			asnRecord["autonomous_system_organization"] = mmdbtype.String(source.ASName)
		}
		if len(asnRecord) != 0 {
			if err := asn.Insert(network, asnRecord); err != nil {
				log.Fatalf("insert ASN %s: %v", network, err)
			}
			asnRecords++
		}
		progress.update(networks)
	}
	if err := iterator.Err(); err != nil {
		log.Fatalf("iterate source: %v", err)
	}

	progress.finish()
	fmt.Printf("Writing %s...\n", *cityOutput)
	if err := writeTree(*cityOutput, city); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Writing %s...\n", *asnOutput)
	if err := writeTree(*asnOutput, asn); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("converted %d networks: %d city records, %d ASN records\n", networks, cityRecords, asnRecords)
}

func countNetworks(db *maxminddb.Reader) (uint64, error) {
	iterator := db.Networks(maxminddb.SkipAliasedNetworks)
	var count uint64
	for iterator.Next() {
		var record ipinfoRecord
		if _, err := iterator.Network(&record); err != nil {
			return 0, err
		}
		count++
	}
	return count, iterator.Err()
}

type progressBar struct {
	total       uint64
	lastPercent int
	started     time.Time
}

func newProgressBar(total uint64) *progressBar {
	bar := &progressBar{total: total, lastPercent: -1, started: time.Now()}
	bar.update(0)
	return bar
}

func (bar *progressBar) update(current uint64) {
	percent := int(current * 100 / bar.total)
	if percent == bar.lastPercent && current != bar.total {
		return
	}
	bar.lastPercent = percent
	const width = 40
	filled := percent * width / 100
	rate := float64(current) / time.Since(bar.started).Seconds()
	fmt.Printf("\r[%s%s] %3d%%  %d/%d  %.0f networks/s",
		strings.Repeat("#", filled),
		strings.Repeat("-", width-filled),
		percent,
		current,
		bar.total,
		rate,
	)
}

func (bar *progressBar) finish() {
	bar.update(bar.total)
	fmt.Println()
}

func writeTree(path string, tree *mmdbwriter.Tree) error {
	temporary := path + ".tmp"
	file, err := os.Create(temporary)
	if err != nil {
		return fmt.Errorf("create %s: %w", temporary, err)
	}
	_, writeErr := tree.WriteTo(file)
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("write %s: %w", temporary, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close %s: %w", temporary, closeErr)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("install %s: %w", path, err)
	}
	return nil
}
