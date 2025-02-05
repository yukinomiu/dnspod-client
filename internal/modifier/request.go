package modifier

type (
	Request struct {
		Domain     string `json:"Domain"`
		SubDomain  string `json:"SubDomain"`
		RecordId   int    `json:"RecordId"`
		RecordLine string `json:"RecordLine"`
		Ttl        int    `json:"Ttl"`
	}
)
