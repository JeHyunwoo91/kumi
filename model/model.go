package model

type Acquisition struct {
	Message    string       `json:"message"`
	Result     ResultTPLGET `json:"result"`
	ReturnCode string       `json:"returnCode"`
}

type AquireReport struct {
	Message    string        `json:"message"`
	Result     ResultTPLPOST `json:"result"`
	ReturnCode string        `json:"returnCode"`
}

type ResultTPLGET struct {
	Count string      `json:"count"`
	List  []MediaInfo `json:"list"`
}

type ResultTPLPOST struct {
	Status string `json:"status"`
}

type MediaInfo struct {
	ContentID string `json:"contentId"`
	Bitrate   string `json:"bitrate"`
	Acquire   string `json:"acquire"`
}

type Meta struct {
	ChannelID string `json:"channelId"`
	ContentID string `json:"contentId"`
	CornerID  string `json:"cornerId"`
	Version   string `json:"version"`
	Bitrate   string `json:"bitrate"`
	FilePath  string `json:"filepath"`
	Acquire   string `json:"acquire"`
	IsUse     string `json:"isUse"`
}

type GetMediaParams struct {
	ChannelID string `json:"channelId"`
	ContentID string `json:"contentId"`
	Acquire   string `json:"acquire"`
}
