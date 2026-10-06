package play

import (
	"log"
	"os"
	"sort"
	"strings"

	global "github.com/OGA45/gomatalk/pkg/global_vars"
	"github.com/OGA45/gomatalk/pkg/voice"
)

// GlobalPlay talk
func GlobalPlay(speechSig chan voice.SpeechSignal) {
	for speech := range speechSig {
		speech.V.PlayQueue(speech.Data)
	}
}

func Exists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func ReplaceWords(guildID string, text *string) error {
	wordList, err := global.DB.ListWords(guildID)
	if err != nil {
		log.Println("ERR: Cannot get word list.")
		return err
	}
	if len(wordList) == 0 {
		return nil
	}

	// Replace longer words first (NewReplacer tries replacements in argument
	// order at each position), and do it in a single pass so substituted
	// output is never re-replaced by a later rule.
	keys := make([]string, 0, len(wordList))
	for k := range wordList {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })

	pairs := make([]string, 0, len(keys)*2)
	for _, k := range keys {
		pairs = append(pairs, k, wordList[k])
	}
	*text = strings.NewReplacer(pairs...).Replace(*text)

	return nil
}
