// Package symbols resolves brokerage security identifiers (tickers, EGX ISINs,
// and company names) into normalized accounting symbols such as EGX:BONY.
package symbols

import (
	"strings"
	"unicode"
)

// builtinISINToTicker maps verified Egyptian Exchange (EGX) ISIN codes to tickers.
var builtinISINToTicker = map[string]string{
	// Workspace PDF & CSV securities
	"EGS656M1C010": "BONY", // Bonyan for Development and Trade
	"EGS380S1C017": "SKPC", // Sidi Kerir Petrochemicals
	"EGS60101C010": "SAUD", // Al Baraka Bank Egypt
	"EGS3E181C010": "EGAL", // Egypt Aluminum
	"EGS72081C010": "AMES", // Alexandria New Medical Center
	"EGS380P1C010": "AMOC", // Alexandria Mineral Oils Company
	"EGS02091C014": "MPCO", // Mansoura Poultry
	"EGS73BR1C013": "SPMD", // Speed Medical
	"EGS30361C017": "MILS", // North Cairo Mills

	// Major EGX30 / EGX70 equities
	"EGS60121C018": "COMI", // Commercial International Bank (CIB)
	"EGS691S1C011": "TMGH", // Talaat Moustafa Group Holding
	"EGS69101C011": "HRHO", // EFG Holding
	"EGS3G0Z1C014": "SWDY", // Elsewedy Electric
	"EGS305I1C011": "EFID", // Edita Food Industries
	"EGS48031C016": "ETEL", // Telecom Egypt
	"EGS38191C010": "ABUK", // Abou Kir Fertilizers
	"EGS745L1C014": "FWRY", // Fawry
	"EGS30841C018": "ORWE", // Oriental Weavers
	"EGS37091C013": "EAST", // Eastern Company
	"EGS655L1C012": "PHDC", // Palm Hills Development
	"EGS39061C014": "MFPC", // Misr Fertilizers Production Company (MOPCO)
	"EGS60111C019": "ADIB", // Abu Dhabi Islamic Bank - Egypt
	"EGS60041C018": "CIEB", // Credit Agricole Egypt
	"EGS60301C016": "HDBK", // Housing and Development Bank
	"EGS30901C010": "JUFO", // Juhayna Food Industries
	"EGS95001C011": "ORAS", // Orascom Construction PLC
	"EGS70321C012": "ORHD", // Orascom Development Egypt
	"EGS65591C017": "HELI", // Heliopolis Housing
	"EGS65571C019": "MASR", // Madinet Masr For Housing and Development
	"EGS673Y1C015": "EMFD", // Emaar Misr for Development
	"EGS743O1C013": "EFIH", // E-finance For Digital and Financial Investments
	"EGS691G1C015": "BTFH", // Beltone Holding
	"EGS42111C012": "ALCN", // Alexandria Containers and Goods
	"EGS673T1C012": "GBCO", // GB Corp
	"EGS729J1C018": "CLHO", // Cleopatra Hospital Company
	"EGS381B1C015": "RMDA", // Tenth of Ramadan Pharmaceutical Industries (Rameda)
	"EGS512O1C012": "ISPH", // Ibnsina Pharma
	"EGS3C251C013": "ESRS", // El Ezz Steel
	"EGS60321C014": "FAIT", // Faisal Islamic Bank of Egypt (EGP)
	"EGS60322C012": "FAITA", // Faisal Islamic Bank of Egypt (USD)
}

// builtinNameToTicker maps normalized security names to EGX tickers.
var builtinNameToTicker = map[string]string{
	"bonyan for development and trade":                         "BONY",
	"sidi kerir petrochemicals":                                "SKPC",
	"sidi kerir petrochemicals co":                             "SKPC",
	"al baraka bank":                                           "SAUD",
	"al baraka bank egypt":                                     "SAUD",
	"egypt aluminum":                                           "EGAL",
	"aluminium company of egypt":                               "EGAL",
	"alexandria new medical center":                            "AMES",
	"alexandria mineral oils company":                          "AMOC",
	"alexandria mineral oils":                                  "AMOC",
	"mansoura poultry":                                         "MPCO",
	"speed medical":                                            "SPMD",
	"north cairo mills":                                        "MILS",
	"north cairo flour mills":                                  "MILS",
	"commercial international bank":                            "COMI",
	"commercial international bank cib":                        "COMI",
	"talaat moustafa group holding":                            "TMGH",
	"talaat moustafa group":                                    "TMGH",
	"efg holding":                                              "HRHO",
	"efg hermes":                                               "HRHO",
	"elsewedy electric":                                        "SWDY",
	"edita food industries":                                    "EFID",
	"telecom egypt":                                            "ETEL",
	"abou kir fertilizers":                                     "ABUK",
	"fawry":                                                    "FWRY",
	"fawry for banking technology and electronic payment":      "FWRY",
	"oriental weavers":                                         "ORWE",
	"eastern company":                                          "EAST",
	"palm hills development":                                   "PHDC",
	"palm hills development company":                           "PHDC",
	"misr fertilizers production company":                      "MFPC",
	"misr fertilizers production company mopco":                "MFPC",
	"abu dhabi islamic bank egypt":                             "ADIB",
	"abu dhabi islamic bank":                                   "ADIB",
	"credit agricole egypt":                                    "CIEB",
	"housing and development bank":                             "HDBK",
	"juhayna food industries":                                  "JUFO",
	"orascom construction":                                     "ORAS",
	"orascom construction plc":                                 "ORAS",
	"orascom development egypt":                                "ORHD",
	"heliopolis housing":                                       "HELI",
	"misr el gadida for housing":                               "HELI",
	"madinet masr for housing and development":                 "MASR",
	"madinet masr":                                             "MASR",
	"emaar misr for development":                               "EMFD",
	"e finance for digital and financial investments":          "EFIH",
	"efinance":                                                 "EFIH",
	"beltone holding":                                          "BTFH",
	"alexandria containers and goods":                          "ALCN",
	"gb corp":                                                  "GBCO",
	"gb auto":                                                  "GBCO",
	"cleopatra hospital":                                       "CLHO",
	"cleopatra hospital company":                               "CLHO",
	"tenth of ramadan pharmaceutical industries and diagnostic": "RMDA",
	"rameda":                                                   "RMDA",
	"ibnsina pharma":                                           "ISPH",
	"el ezz steel":                                             "ESRS",
	"ezz steel":                                                "ESRS",
	"faisal islamic bank of egypt":                             "FAIT",
}

// NormalizeName canonicalizes company names for resilient dictionary matching.
func NormalizeName(s string) string {
	var b strings.Builder
	prevSpace := true
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevSpace = false
		} else if !prevSpace {
			b.WriteByte(' ')
			prevSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}
