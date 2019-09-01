package configs

// RabbitSeasoning represents the configuration values.
type RabbitSeasoning struct {
	ConnectionFactory struct {
		URI string `json:"URI"`
	} `json:"ConnectionFactory"`
	TLSConfig struct {
		EnableTLS         bool   `json:"EnableTLS"`
		PEMCertLocation   string `json:"PEMCertLocation"`
		LocalCertLocation string `json:"LocalCertLocation"`
		CertServerName    string `json:"CertServerName"`
	} `json:"TLSConfig"`
}
