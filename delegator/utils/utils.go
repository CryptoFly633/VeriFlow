package utils

import (
	"encoding/base64"
	"fmt"

	abci "github.com/cometbft/cometbft/abci/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func ParseBase64Event(event abci.Event) sdk.StringEvent {
	evt := sdk.StringEvent{Type: event.Type}
	for _, attr := range event.Attributes {
		key, err := base64.StdEncoding.DecodeString(attr.Key)
		if err != nil {
			fmt.Println("Failed to decode legacy key as base64", err)
			continue
		}
		value, err := base64.StdEncoding.DecodeString(attr.Value)
		if err != nil {
			fmt.Println("Failed to decode legacy value as base64", err)
			continue
		}
		evt.Attributes = append(evt.Attributes, sdk.Attribute{
			Key:   string(key),
			Value: string(value),
		})
	}
	return evt
}

func ParseBase64Events(events []abci.Event) []sdk.StringEvent {
	var evts []sdk.StringEvent
	for _, event := range events {
		evt := sdk.StringEvent{Type: event.Type}
		for _, attr := range event.Attributes {
			key, err := base64.StdEncoding.DecodeString(attr.Key)
			if err != nil {
				fmt.Println("Failed to decode legacy key as base64", err)
				continue
			}
			value, err := base64.StdEncoding.DecodeString(attr.Value)
			if err != nil {
				fmt.Println("Failed to decode legacy value as base64", err)
				continue
			}
			evt.Attributes = append(evt.Attributes, sdk.Attribute{
				Key:   string(key),
				Value: string(value),
			})
		}
		evts = append(evts, evt)
	}
	return evts
}
