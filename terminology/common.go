package terminology

// loadCommonCodeSystems loads commonly used FHIR code systems.
func (s *InMemoryTerminologyService) loadCommonCodeSystems() {
	// ISO 3166-1 Country Codes (numeric and alpha-2)
	// This is publicly available data from the ISO 3166 standard
	s.addCodeSystem("urn:iso:std:iso:3166", map[string]string{
		// Numeric codes (3-digit)
		"004": "Afghanistan",
		"008": "Albania",
		"012": "Algeria",
		"016": "American Samoa",
		"020": "Andorra",
		"024": "Angola",
		"028": "Antigua and Barbuda",
		"031": "Azerbaijan",
		"032": "Argentina",
		"036": "Australia",
		"040": "Austria",
		"044": "Bahamas",
		"048": "Bahrain",
		"050": "Bangladesh",
		"051": "Armenia",
		"052": "Barbados",
		"056": "Belgium",
		"060": "Bermuda",
		"064": "Bhutan",
		"068": "Bolivia",
		"070": "Bosnia and Herzegovina",
		"072": "Botswana",
		"076": "Brazil",
		"084": "Belize",
		"090": "Solomon Islands",
		"092": "Virgin Islands (British)",
		"096": "Brunei Darussalam",
		"100": "Bulgaria",
		"104": "Myanmar",
		"108": "Burundi",
		"112": "Belarus",
		"116": "Cambodia",
		"120": "Cameroon",
		"124": "Canada",
		"132": "Cabo Verde",
		"136": "Cayman Islands",
		"140": "Central African Republic",
		"144": "Sri Lanka",
		"148": "Chad",
		"152": "Chile",
		"156": "China",
		"158": "Taiwan",
		"162": "Christmas Island",
		"166": "Cocos (Keeling) Islands",
		"170": "Colombia",
		"174": "Comoros",
		"175": "Mayotte",
		"178": "Congo",
		"180": "Congo (Democratic Republic)",
		"184": "Cook Islands",
		"188": "Costa Rica",
		"191": "Croatia",
		"192": "Cuba",
		"196": "Cyprus",
		"203": "Czechia",
		"204": "Benin",
		"208": "Denmark",
		"212": "Dominica",
		"214": "Dominican Republic",
		"218": "Ecuador",
		"222": "El Salvador",
		"226": "Equatorial Guinea",
		"231": "Ethiopia",
		"232": "Eritrea",
		"233": "Estonia",
		"234": "Faroe Islands",
		"238": "Falkland Islands",
		"242": "Fiji",
		"246": "Finland",
		"250": "France",
		"254": "French Guiana",
		"258": "French Polynesia",
		"262": "Djibouti",
		"266": "Gabon",
		"268": "Georgia",
		"270": "Gambia",
		"275": "Palestine",
		"276": "Germany",
		"288": "Ghana",
		"292": "Gibraltar",
		"296": "Kiribati",
		"300": "Greece",
		"304": "Greenland",
		"308": "Grenada",
		"312": "Guadeloupe",
		"316": "Guam",
		"320": "Guatemala",
		"324": "Guinea",
		"328": "Guyana",
		"332": "Haiti",
		"336": "Holy See",
		"340": "Honduras",
		"344": "Hong Kong",
		"348": "Hungary",
		"352": "Iceland",
		"356": "India",
		"360": "Indonesia",
		"364": "Iran",
		"368": "Iraq",
		"372": "Ireland",
		"376": "Israel",
		"380": "Italy",
		"384": "Côte d'Ivoire",
		"388": "Jamaica",
		"392": "Japan",
		"398": "Kazakhstan",
		"400": "Jordan",
		"404": "Kenya",
		"408": "Korea (Democratic People's Republic)",
		"410": "Korea (Republic)",
		"414": "Kuwait",
		"417": "Kyrgyzstan",
		"418": "Lao People's Democratic Republic",
		"422": "Lebanon",
		"426": "Lesotho",
		"428": "Latvia",
		"430": "Liberia",
		"434": "Libya",
		"438": "Liechtenstein",
		"440": "Lithuania",
		"442": "Luxembourg",
		"446": "Macao",
		"450": "Madagascar",
		"454": "Malawi",
		"458": "Malaysia",
		"462": "Maldives",
		"466": "Mali",
		"470": "Malta",
		"474": "Martinique",
		"478": "Mauritania",
		"480": "Mauritius",
		"484": "Mexico",
		"492": "Monaco",
		"496": "Mongolia",
		"498": "Moldova",
		"499": "Montenegro",
		"500": "Montserrat",
		"504": "Morocco",
		"508": "Mozambique",
		"512": "Oman",
		"516": "Namibia",
		"520": "Nauru",
		"524": "Nepal",
		"528": "Netherlands",
		"531": "Curaçao",
		"533": "Aruba",
		"534": "Sint Maarten",
		"535": "Bonaire, Sint Eustatius and Saba",
		"540": "New Caledonia",
		"548": "Vanuatu",
		"554": "New Zealand",
		"558": "Nicaragua",
		"562": "Niger",
		"566": "Nigeria",
		"570": "Niue",
		"574": "Norfolk Island",
		"578": "Norway",
		"580": "Northern Mariana Islands",
		"583": "Micronesia",
		"584": "Marshall Islands",
		"585": "Palau",
		"586": "Pakistan",
		"591": "Panama",
		"598": "Papua New Guinea",
		"600": "Paraguay",
		"604": "Peru",
		"608": "Philippines",
		"612": "Pitcairn",
		"616": "Poland",
		"620": "Portugal",
		"624": "Guinea-Bissau",
		"626": "Timor-Leste",
		"630": "Puerto Rico",
		"634": "Qatar",
		"638": "Réunion",
		"642": "Romania",
		"643": "Russian Federation",
		"646": "Rwanda",
		"652": "Saint Barthélemy",
		"654": "Saint Helena",
		"659": "Saint Kitts and Nevis",
		"660": "Anguilla",
		"662": "Saint Lucia",
		"663": "Saint Martin",
		"666": "Saint Pierre and Miquelon",
		"670": "Saint Vincent and the Grenadines",
		"674": "San Marino",
		"678": "Sao Tome and Principe",
		"682": "Saudi Arabia",
		"686": "Senegal",
		"688": "Serbia",
		"690": "Seychelles",
		"694": "Sierra Leone",
		"702": "Singapore",
		"703": "Slovakia",
		"704": "Viet Nam",
		"705": "Slovenia",
		"706": "Somalia",
		"710": "South Africa",
		"716": "Zimbabwe",
		"724": "Spain",
		"728": "South Sudan",
		"729": "Sudan",
		"732": "Western Sahara",
		"740": "Suriname",
		"744": "Svalbard and Jan Mayen",
		"748": "Eswatini",
		"752": "Sweden",
		"756": "Switzerland",
		"760": "Syrian Arab Republic",
		"762": "Tajikistan",
		"764": "Thailand",
		"768": "Togo",
		"772": "Tokelau",
		"776": "Tonga",
		"780": "Trinidad and Tobago",
		"784": "United Arab Emirates",
		"788": "Tunisia",
		"792": "Türkiye",
		"795": "Turkmenistan",
		"796": "Turks and Caicos Islands",
		"798": "Tuvalu",
		"800": "Uganda",
		"804": "Ukraine",
		"807": "North Macedonia",
		"818": "Egypt",
		"826": "United Kingdom",
		"831": "Guernsey",
		"832": "Jersey",
		"833": "Isle of Man",
		"834": "Tanzania",
		"840": "United States of America",
		"850": "Virgin Islands (U.S.)",
		"854": "Burkina Faso",
		"858": "Uruguay",
		"860": "Uzbekistan",
		"862": "Venezuela",
		"876": "Wallis and Futuna",
		"882": "Samoa",
		"887": "Yemen",
		"894": "Zambia",
		// Alpha-2 codes (2-letter)
		"AF": "Afghanistan",
		"AL": "Albania",
		"DZ": "Algeria",
		"AS": "American Samoa",
		"AD": "Andorra",
		"AO": "Angola",
		"AG": "Antigua and Barbuda",
		"AZ": "Azerbaijan",
		"AR": "Argentina",
		"AU": "Australia",
		"AT": "Austria",
		"BS": "Bahamas",
		"BH": "Bahrain",
		"BD": "Bangladesh",
		"AM": "Armenia",
		"BB": "Barbados",
		"BE": "Belgium",
		"BM": "Bermuda",
		"BT": "Bhutan",
		"BO": "Bolivia",
		"BA": "Bosnia and Herzegovina",
		"BW": "Botswana",
		"BR": "Brazil",
		"BZ": "Belize",
		"SB": "Solomon Islands",
		"VG": "Virgin Islands (British)",
		"BN": "Brunei Darussalam",
		"BG": "Bulgaria",
		"MM": "Myanmar",
		"BI": "Burundi",
		"BY": "Belarus",
		"KH": "Cambodia",
		"CM": "Cameroon",
		"CA": "Canada",
		"CV": "Cabo Verde",
		"KY": "Cayman Islands",
		"CF": "Central African Republic",
		"LK": "Sri Lanka",
		"TD": "Chad",
		"CL": "Chile",
		"CN": "China",
		"TW": "Taiwan",
		"CX": "Christmas Island",
		"CC": "Cocos (Keeling) Islands",
		"CO": "Colombia",
		"KM": "Comoros",
		"YT": "Mayotte",
		"CG": "Congo",
		"CD": "Congo (Democratic Republic)",
		"CK": "Cook Islands",
		"CR": "Costa Rica",
		"HR": "Croatia",
		"CU": "Cuba",
		"CY": "Cyprus",
		"CZ": "Czechia",
		"BJ": "Benin",
		"DK": "Denmark",
		"DM": "Dominica",
		"DO": "Dominican Republic",
		"EC": "Ecuador",
		"SV": "El Salvador",
		"GQ": "Equatorial Guinea",
		"ET": "Ethiopia",
		"ER": "Eritrea",
		"EE": "Estonia",
		"FO": "Faroe Islands",
		"FK": "Falkland Islands",
		"FJ": "Fiji",
		"FI": "Finland",
		"FR": "France",
		"GF": "French Guiana",
		"PF": "French Polynesia",
		"DJ": "Djibouti",
		"GA": "Gabon",
		"GE": "Georgia",
		"GM": "Gambia",
		"PS": "Palestine",
		"DE": "Germany",
		"GH": "Ghana",
		"GI": "Gibraltar",
		"KI": "Kiribati",
		"GR": "Greece",
		"GL": "Greenland",
		"GD": "Grenada",
		"GP": "Guadeloupe",
		"GU": "Guam",
		"GT": "Guatemala",
		"GN": "Guinea",
		"GY": "Guyana",
		"HT": "Haiti",
		"VA": "Holy See",
		"HN": "Honduras",
		"HK": "Hong Kong",
		"HU": "Hungary",
		"IS": "Iceland",
		"IN": "India",
		"ID": "Indonesia",
		"IR": "Iran",
		"IQ": "Iraq",
		"IE": "Ireland",
		"IL": "Israel",
		"IT": "Italy",
		"CI": "Côte d'Ivoire",
		"JM": "Jamaica",
		"JP": "Japan",
		"KZ": "Kazakhstan",
		"JO": "Jordan",
		"KE": "Kenya",
		"KP": "Korea (Democratic People's Republic)",
		"KR": "Korea (Republic)",
		"KW": "Kuwait",
		"KG": "Kyrgyzstan",
		"LA": "Lao People's Democratic Republic",
		"LB": "Lebanon",
		"LS": "Lesotho",
		"LV": "Latvia",
		"LR": "Liberia",
		"LY": "Libya",
		"LI": "Liechtenstein",
		"LT": "Lithuania",
		"LU": "Luxembourg",
		"MO": "Macao",
		"MG": "Madagascar",
		"MW": "Malawi",
		"MY": "Malaysia",
		"MV": "Maldives",
		"ML": "Mali",
		"MT": "Malta",
		"MQ": "Martinique",
		"MR": "Mauritania",
		"MU": "Mauritius",
		"MX": "Mexico",
		"MC": "Monaco",
		"MN": "Mongolia",
		"MD": "Moldova",
		"ME": "Montenegro",
		"MS": "Montserrat",
		"MA": "Morocco",
		"MZ": "Mozambique",
		"OM": "Oman",
		"NA": "Namibia",
		"NR": "Nauru",
		"NP": "Nepal",
		"NL": "Netherlands",
		"CW": "Curaçao",
		"AW": "Aruba",
		"SX": "Sint Maarten",
		"BQ": "Bonaire, Sint Eustatius and Saba",
		"NC": "New Caledonia",
		"VU": "Vanuatu",
		"NZ": "New Zealand",
		"NI": "Nicaragua",
		"NE": "Niger",
		"NG": "Nigeria",
		"NU": "Niue",
		"NF": "Norfolk Island",
		"NO": "Norway",
		"MP": "Northern Mariana Islands",
		"FM": "Micronesia",
		"MH": "Marshall Islands",
		"PW": "Palau",
		"PK": "Pakistan",
		"PA": "Panama",
		"PG": "Papua New Guinea",
		"PY": "Paraguay",
		"PE": "Peru",
		"PH": "Philippines",
		"PN": "Pitcairn",
		"PL": "Poland",
		"PT": "Portugal",
		"GW": "Guinea-Bissau",
		"TL": "Timor-Leste",
		"PR": "Puerto Rico",
		"QA": "Qatar",
		"RE": "Réunion",
		"RO": "Romania",
		"RU": "Russian Federation",
		"RW": "Rwanda",
		"BL": "Saint Barthélemy",
		"SH": "Saint Helena",
		"KN": "Saint Kitts and Nevis",
		"AI": "Anguilla",
		"LC": "Saint Lucia",
		"MF": "Saint Martin",
		"PM": "Saint Pierre and Miquelon",
		"VC": "Saint Vincent and the Grenadines",
		"SM": "San Marino",
		"ST": "Sao Tome and Principe",
		"SA": "Saudi Arabia",
		"SN": "Senegal",
		"RS": "Serbia",
		"SC": "Seychelles",
		"SL": "Sierra Leone",
		"SG": "Singapore",
		"SK": "Slovakia",
		"VN": "Viet Nam",
		"SI": "Slovenia",
		"SO": "Somalia",
		"ZA": "South Africa",
		"ZW": "Zimbabwe",
		"ES": "Spain",
		"SS": "South Sudan",
		"SD": "Sudan",
		"EH": "Western Sahara",
		"SR": "Suriname",
		"SJ": "Svalbard and Jan Mayen",
		"SZ": "Eswatini",
		"SE": "Sweden",
		"CH": "Switzerland",
		"SY": "Syrian Arab Republic",
		"TJ": "Tajikistan",
		"TH": "Thailand",
		"TG": "Togo",
		"TK": "Tokelau",
		"TO": "Tonga",
		"TT": "Trinidad and Tobago",
		"AE": "United Arab Emirates",
		"TN": "Tunisia",
		"TR": "Türkiye",
		"TM": "Turkmenistan",
		"TC": "Turks and Caicos Islands",
		"TV": "Tuvalu",
		"UG": "Uganda",
		"UA": "Ukraine",
		"MK": "North Macedonia",
		"EG": "Egypt",
		"GB": "United Kingdom",
		"GG": "Guernsey",
		"JE": "Jersey",
		"IM": "Isle of Man",
		"TZ": "Tanzania",
		"US": "United States of America",
		"VI": "Virgin Islands (U.S.)",
		"BF": "Burkina Faso",
		"UY": "Uruguay",
		"UZ": "Uzbekistan",
		"VE": "Venezuela",
		"WF": "Wallis and Futuna",
		"WS": "Samoa",
		"YE": "Yemen",
		"ZM": "Zambia",
	})

	// Administrative Gender
	s.addCodeSystem("http://hl7.org/fhir/administrative-gender", map[string]string{
		"male":    "Male",
		"female":  "Female",
		"other":   "Other",
		"unknown": "Unknown",
	})

	// Yes/No
	s.addCodeSystem("http://terminology.hl7.org/CodeSystem/v2-0136", map[string]string{
		"Y": "Yes",
		"N": "No",
	})

	// Contact Point System
	s.addCodeSystem("http://hl7.org/fhir/contact-point-system", map[string]string{
		"phone": "Phone",
		"fax":   "Fax",
		"email": "Email",
		"pager": "Pager",
		"url":   "URL",
		"sms":   "SMS",
		"other": "Other",
	})

	// Contact Point Use
	s.addCodeSystem("http://hl7.org/fhir/contact-point-use", map[string]string{
		"home":   "Home",
		"work":   "Work",
		"temp":   "Temp",
		"old":    "Old",
		"mobile": "Mobile",
	})

	// Name Use
	s.addCodeSystem("http://hl7.org/fhir/name-use", map[string]string{
		"usual":     "Usual",
		"official":  "Official",
		"temp":      "Temp",
		"nickname":  "Nickname",
		"anonymous": "Anonymous",
		"old":       "Old",
		"maiden":    "Maiden",
	})

	// Address Use
	s.addCodeSystem("http://hl7.org/fhir/address-use", map[string]string{
		"home":    "Home",
		"work":    "Work",
		"temp":    "Temporary",
		"old":     "Old / Incorrect",
		"billing": "Billing",
	})

	// Address Type
	s.addCodeSystem("http://hl7.org/fhir/address-type", map[string]string{
		"postal":   "Postal",
		"physical": "Physical",
		"both":     "Postal & Physical",
	})

	// Identifier Use
	s.addCodeSystem("http://hl7.org/fhir/identifier-use", map[string]string{
		"usual":     "Usual",
		"official":  "Official",
		"temp":      "Temp",
		"secondary": "Secondary",
		"old":       "Old",
	})

	// Observation Status
	s.addCodeSystem("http://hl7.org/fhir/observation-status", map[string]string{
		"registered":    "Registered",
		"preliminary":   "Preliminary",
		"final":         "Final",
		"amended":       "Amended",
		"corrected":     "Corrected",
		"cancelled":     "Cancelled",
		"entered-in-error": "Entered in Error",
		"unknown":       "Unknown",
	})

	// Bundle Type
	s.addCodeSystem("http://hl7.org/fhir/bundle-type", map[string]string{
		"document":          "Document",
		"message":           "Message",
		"transaction":       "Transaction",
		"transaction-response": "Transaction Response",
		"batch":             "Batch",
		"batch-response":    "Batch Response",
		"history":           "History List",
		"searchset":         "Search Results",
		"collection":        "Collection",
	})

	// HTTP Verb
	s.addCodeSystem("http://hl7.org/fhir/http-verb", map[string]string{
		"GET":    "GET",
		"HEAD":   "HEAD",
		"POST":   "POST",
		"PUT":    "PUT",
		"DELETE": "DELETE",
		"PATCH":  "PATCH",
	})

	// Publication Status
	s.addCodeSystem("http://hl7.org/fhir/publication-status", map[string]string{
		"draft":   "Draft",
		"active":  "Active",
		"retired": "Retired",
		"unknown": "Unknown",
	})

	// Request Status
	s.addCodeSystem("http://hl7.org/fhir/request-status", map[string]string{
		"draft":          "Draft",
		"active":         "Active",
		"on-hold":        "On Hold",
		"revoked":        "Revoked",
		"completed":      "Completed",
		"entered-in-error": "Entered in Error",
		"unknown":        "Unknown",
	})

	// Condition Clinical Status
	s.addCodeSystem("http://terminology.hl7.org/CodeSystem/condition-clinical", map[string]string{
		"active":     "Active",
		"recurrence": "Recurrence",
		"relapse":    "Relapse",
		"inactive":   "Inactive",
		"remission":  "Remission",
		"resolved":   "Resolved",
	})

	// Condition Verification Status
	s.addCodeSystem("http://terminology.hl7.org/CodeSystem/condition-ver-status", map[string]string{
		"unconfirmed":      "Unconfirmed",
		"provisional":      "Provisional",
		"differential":     "Differential",
		"confirmed":        "Confirmed",
		"refuted":          "Refuted",
		"entered-in-error": "Entered in Error",
	})

	// Create common ValueSets that reference these code systems
	s.addValueSetFromCodeSystem(
		"http://hl7.org/fhir/ValueSet/administrative-gender",
		"http://hl7.org/fhir/administrative-gender",
	)
	s.addValueSetFromCodeSystem(
		"http://hl7.org/fhir/ValueSet/contact-point-system",
		"http://hl7.org/fhir/contact-point-system",
	)
	s.addValueSetFromCodeSystem(
		"http://hl7.org/fhir/ValueSet/contact-point-use",
		"http://hl7.org/fhir/contact-point-use",
	)
	s.addValueSetFromCodeSystem(
		"http://hl7.org/fhir/ValueSet/name-use",
		"http://hl7.org/fhir/name-use",
	)
	s.addValueSetFromCodeSystem(
		"http://hl7.org/fhir/ValueSet/address-use",
		"http://hl7.org/fhir/address-use",
	)
	s.addValueSetFromCodeSystem(
		"http://hl7.org/fhir/ValueSet/address-type",
		"http://hl7.org/fhir/address-type",
	)
	s.addValueSetFromCodeSystem(
		"http://hl7.org/fhir/ValueSet/identifier-use",
		"http://hl7.org/fhir/identifier-use",
	)
	s.addValueSetFromCodeSystem(
		"http://hl7.org/fhir/ValueSet/observation-status",
		"http://hl7.org/fhir/observation-status",
	)
	s.addValueSetFromCodeSystem(
		"http://hl7.org/fhir/ValueSet/bundle-type",
		"http://hl7.org/fhir/bundle-type",
	)
	s.addValueSetFromCodeSystem(
		"http://hl7.org/fhir/ValueSet/http-verb",
		"http://hl7.org/fhir/http-verb",
	)
	s.addValueSetFromCodeSystem(
		"http://hl7.org/fhir/ValueSet/publication-status",
		"http://hl7.org/fhir/publication-status",
	)
}

// addCodeSystem adds a simple code system to the terminology service.
func (s *InMemoryTerminologyService) addCodeSystem(url string, codes map[string]string) {
	csData := &codeSystemData{
		url:   url,
		codes: make(map[string]codeEntry, len(codes)),
	}

	for code, display := range codes {
		csData.codes[code] = codeEntry{
			code:    code,
			display: display,
			system:  url,
		}
	}

	s.codeSystems[url] = csData
}

// addValueSetFromCodeSystem creates a ValueSet that includes all codes from a CodeSystem.
func (s *InMemoryTerminologyService) addValueSetFromCodeSystem(vsURL, csURL string) {
	cs, ok := s.codeSystems[csURL]
	if !ok {
		return
	}

	vsData := &valueSetData{
		url:   vsURL,
		codes: make(map[string]map[string]codeEntry),
	}

	vsData.codes[csURL] = make(map[string]codeEntry, len(cs.codes))
	for code, entry := range cs.codes {
		vsData.codes[csURL][code] = entry
	}

	s.valueSets[vsURL] = vsData
}

// AddCustomValueSet adds a custom ValueSet with explicit codes.
func (s *InMemoryTerminologyService) AddCustomValueSet(url, system string, codes map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	vsData := &valueSetData{
		url:   url,
		codes: make(map[string]map[string]codeEntry),
	}

	vsData.codes[system] = make(map[string]codeEntry, len(codes))
	for code, display := range codes {
		vsData.codes[system][code] = codeEntry{
			code:    code,
			display: display,
			system:  system,
		}
	}

	s.valueSets[url] = vsData
}

// AddCustomCodeSystem adds a custom CodeSystem.
func (s *InMemoryTerminologyService) AddCustomCodeSystem(url string, codes map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addCodeSystem(url, codes)
}
