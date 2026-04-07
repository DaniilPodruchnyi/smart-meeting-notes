package telegram

import "gopkg.in/telebot.v3"

type BotHandler func(ctx telebot.Context) error

type handlers struct {
	onText  BotHandler
	onVoice BotHandler
	onAudio BotHandler
	onStart BotHandler
	onList  BotHandler
	onGet   BotHandler
	onFind  BotHandler
	onChat  BotHandler
}

func (h *handlers) Text(f BotHandler)  { h.onText = f }
func (h *handlers) Voice(f BotHandler) { h.onVoice = f }
func (h *handlers) Audio(f BotHandler) { h.onAudio = f }
func (h *handlers) Start(f BotHandler) { h.onStart = f }
func (h *handlers) List(f BotHandler)  { h.onList = f }
func (h *handlers) Get(f BotHandler)   { h.onGet = f }
func (h *handlers) Find(f BotHandler)  { h.onFind = f }
func (h *handlers) Chat(f BotHandler)  { h.onChat = f }
