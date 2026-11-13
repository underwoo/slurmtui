package ui

type mode uint8

const (
	modeList mode = iota
	modeDetail
	modeFilter
	modePicker
	modeConfirm
	modeForm
)
