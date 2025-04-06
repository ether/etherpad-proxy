package models

type Settings struct {
	Port               int                `json:"port"`
	Backends           map[string]Backend `json:"backends"`
	MaxPadsPerInstance int                `json:"maxPadsPerInstance"`
	CheckInterval      int64              `json:"checkInterval"`
	DBSettings         DBSettings         `json:"dbSettings"`
}

type DBSettings struct {
	Filename string `json:"filename"`
}

type Backend struct {
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	ClientId     *string  `json:"clientId"`
	ClientSecret *string  `json:"clientSecret"`
	Scopes       []string `json:"scopes"`
	TokenURL     *string  `json:"tokenUrl"`
	Username     *string  `json:"username"`
	Password     *string  `json:"password"`
}
