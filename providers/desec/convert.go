package desec

// Convert the provider's native record description to models.RecordConfig.

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/StackExchange/dnscontrol/v3/models"
	"github.com/StackExchange/dnscontrol/v3/pkg/printer"
)

// nativeToRecord takes a DNS record from deSEC and returns a native RecordConfig struct.
func nativeToRecords(n resourceRecord, origin string) (rcs []*models.RecordConfig) {

	// deSEC returns all the values for a given label/rtype pair in each
	// resourceRecord.  In other words, if there are multiple A
	// records for a label, all the IP addresses are listed in
	// n.Records rather than having many resourceRecord's.
	// We must split them out into individual records, one for each value.
	for _, value := range n.Records {
		rc := &models.RecordConfig{
			TTL:      n.TTL,
			Original: n,
		}
		rc.SetLabel(n.Subname, origin)
		switch rtype := n.Type; rtype {
		case "TXT":
			rc.SetTargetTXTs(decodeTxt(value))
		default: //  "A", "AAAA", "CAA", "NS", "CNAME", "MX", "PTR", "SRV"
			if err := rc.PopulateFromString(rtype, value, origin); err != nil {
				panic(fmt.Errorf("unparsable record received from deSEC: %w", err))
			}
		}
		rcs = append(rcs, rc)
	}

	return rcs
}

func recordsToNative(rcs []*models.RecordConfig, origin string) []resourceRecord {
	// Take a list of RecordConfig and return an equivalent list of resourceRecord.
	// deSEC requires one resourceRecord for each label:key tuple, therefore we
	// might collapse many RecordConfig into one resourceRecord.

	var keys = map[models.RecordKey]*resourceRecord{}
	var zrs []resourceRecord

	for _, r := range rcs {
		label := r.GetLabel()
		if label == "@" {
			label = ""
		}
		key := r.Key()

		if zr, ok := keys[key]; !ok {
			// Allocate a new ZoneRecord:
			zr := resourceRecord{
				Type:    r.Type,
				TTL:     r.TTL,
				Subname: label,
				Records: []string{r.GetTargetCombined()},
			}
			if r.Type == "TXT" {
				zr.Records = []string{r.GetTargetField()}
			}
			zrs = append(zrs, zr)
			//keys[key] = &zr   // This didn't work.
			keys[key] = &zrs[len(zrs)-1] // This does work. I don't know why.

		} else {
			zr.Records = append(zr.Records, r.GetTargetCombined())

			if r.TTL != zr.TTL {
				printer.Warnf("All TTLs for a rrset (%v) must be the same. Using smaller of %v and %v.\n", key, r.TTL, zr.TTL)
				if r.TTL < zr.TTL {
					zr.TTL = r.TTL
				}
			}

		}
	}

	return zrs
}

// // encodeTxt encodes TxtStrings for sending to the API:
// func encodeTxt(txts []string) string {
// 	ans := txts[0]

// 	if len(txts) > 1 {
// 		ans = ""
// 		for _, t := range txts {
// 			ans += `"` + strings.Replace(t, `"`, `\"`, -1) + `"`
// 		}
// 	}
// 	return ans
// }

// finds a string surrounded by quotes that might contain an escaped quote character.
var quotedStringRegexp = regexp.MustCompile(`"((?:[^"\\]|\\.)*)"`)

// decodeTxt decodes the TXT record as received from the API and
// returns the list of strings.
func decodeTxt(s string) []string {

	// Not encoded:
	if len(s) < 2 || !(s[0] == '"' && s[len(s)-1] == '"') {
		return []string{s}
	}

	txtStrings := []string{}
	for _, t := range quotedStringRegexp.FindAllStringSubmatch(s, -1) {
		txtString := strings.Replace(t[1], `\"`, `"`, -1)
		txtStrings = append(txtStrings, txtString)
	}
	return txtStrings
}
