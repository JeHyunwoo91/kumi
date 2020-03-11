package model

// Acquisition ...
type Acquisition struct {
	Message    string       `json:"message"`
	Result     ResultTplGet `json:"result"`
	ReturnCode string       `json:"returnCode"`
}

// AcquireReport ...
type AcquireReport struct {
	Message    string        `json:"message"`
	Result     ResultTplPost `json:"result"`
	ReturnCode string        `json:"returnCode"`
}

// ResultTplGet ...
type ResultTplGet struct {
	Count string      `json:"count"`
	List  []MediaInfo `json:"list"`
}

// ResultTplPost ...
type ResultTplPost struct {
	Status string `json:"status"`
}

// MediaInfo ...
type MediaInfo struct {
	ContentID string `json:"contentId"`
	Bitrate   string `json:"bitrate"`
	Acquire   string `json:"acquire"`
}

// Meta ...
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

// GetMediaParams ...
type GetMediaParams struct {
	ChannelID string `json:"channelId"`
	ContentID string `json:"contentId"`
	Acquire   string `json:"acquire"`
}
