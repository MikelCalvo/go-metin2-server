package accountstore

import (
	"encoding/json"
	"errors"
	"fmt"
)

// encodeAccountWithChannels keeps the ticket/packet character shape unchanged.
// Channel identity belongs to the committed account snapshot only.
func encodeAccountWithChannels(account Account) ([]byte, error) {
	outer, err := json.Marshal(account)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(outer, &fields); err != nil {
		return nil, err
	}
	characters := make([]json.RawMessage, 0, len(account.Characters))
	for _, character := range account.Characters {
		raw, err := json.Marshal(character)
		if err != nil {
			return nil, err
		}
		if character.ID != 0 {
			var entry map[string]json.RawMessage
			if err := json.Unmarshal(raw, &entry); err != nil {
				return nil, err
			}
			channelRaw, err := json.Marshal(account.ChannelForCharacter(character.ID))
			if err != nil {
				return nil, err
			}
			entry["channel_id"] = channelRaw
			raw, err = json.Marshal(entry)
			if err != nil {
				return nil, err
			}
		}
		characters = append(characters, raw)
	}
	if account.Characters != nil {
		fields["characters"], err = json.Marshal(characters)
		if err != nil {
			return nil, err
		}
	}
	return json.Marshal(fields)
}

// preserveChannelAssignments merges an existing committed channel when callers
// save a loginticket character (which has no channel field). Deletion removes
// the assignment; replacement by a different ID defaults to local channel 1.
func preserveChannelAssignments(account *Account, previous Account) {
	if account.CharacterChannels == nil {
		account.CharacterChannels = make(map[uint32]uint8)
	}
	for _, character := range account.Characters {
		if character.ID == 0 {
			continue
		}
		if _, specified := account.CharacterChannels[character.ID]; specified {
			continue
		}
		for _, old := range previous.Characters {
			if old.ID == character.ID {
				account.CharacterChannels[character.ID] = previous.ChannelForCharacter(character.ID)
				break
			}
		}
	}
}

func (s *FileStore) preserveCharacterChannels(account *Account) error {
	previous, err := s.Load(account.Login)
	if errors.Is(err, ErrAccountNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("load existing account channels: %w", err)
	}
	preserveChannelAssignments(account, previous)
	return nil
}
