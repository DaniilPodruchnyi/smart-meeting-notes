package salutespeech

type uploadResult struct {
	RequestFileID string `json:"request_file_id"`
}

type recognizeResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type statusResult struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	ResponseFileID string `json:"response_file_id"`
}

type apiEnvelope[T any] struct {
	Status int    `json:"status"`
	Result T      `json:"result"`
	Error  string `json:"error"`
}

type recognizeRequest struct {
	Options       recognizeOptions `json:"options"`
	RequestFileID string           `json:"request_file_id"`
}

type recognizeOptions struct {
	Model                    string                    `json:"model"`
	Language                 string                    `json:"language"`
	AudioEncoding            string                    `json:"audio_encoding"`
	SampleRate               int                       `json:"sample_rate"`
	ChannelsCount            int                       `json:"channels_count"`
	SpeakerSeparationOptions *speakerSeparationOptions `json:"speaker_separation_options,omitempty"`
}

type speakerSeparationOptions struct {
	Enable                bool `json:"enable"`
	EnableOnlyMainSpeaker bool `json:"enable_only_main_speaker"`
	Count                 int  `json:"count"`
}

type statusRequest struct {
	ID string `json:"id"`
}

type downloadRequest struct {
	ResponseFileID string `json:"response_file_id"`
}
