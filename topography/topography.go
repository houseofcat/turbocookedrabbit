package topography

// Exchange allows for you to create Exchange topology.
type Exchange struct {
	ExchangeName          string `json:"ExchangeName"`
	ParentExchangeBinding string `json:"ParentExchangeBinding"`
	ExchangeType          string `json:"ExchangeType"`
}

// Queue allows for you to create Queue topology.
type Queue struct {
	QueueName       string `json:"QueueName"`
	PurgeIfExists   bool   `json:"PurgeIfExist"`
	ExchangeBinding string `json:"ExchangeBinding"`
}
