package models

type ListAllPadsModel struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		PadIds []string `json:"padIDs"`
	} `json:"data"`
}
