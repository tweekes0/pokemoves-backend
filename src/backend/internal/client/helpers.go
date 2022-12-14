package client

import (
	"encoding/csv"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// interface for for structs that receive data from api
type APIReceiver interface {
	Init(int)
	AddWorker()
	PostProcess()
	FetchEntries(string, string, int)
	Wait()
	CsvEntries() []CsvEntry
	GetEndpoint() string
	GetEntries() []interface{}
	GetRelations() []interface{}
}

// interface for writing structs to CSV files
type CsvEntry interface {
	GetHeader() []string
	ToSlice() []string
}

// resolves the pokeapi version group to a generation number
// https://pokeapi.co/docs/v2#versiongroup
func resolveVersionGroup(url string) int {
	id := getUrlID(url)

	switch id {
	case 1, 2:
		return 1
	case 3, 4:
		return 2
	case 5, 6, 7, 12, 13:
		return 3
	case 8, 9, 10:
		return 4
	case 11, 14:
		return 5
	case 15, 16:
		return 6
	case 17, 18, 19:
		return 7
	case 20, 21, 22, 23, 24:
		return 8
	default:
		return -1
	}
}

func sanitizeString(s string) string {
	ret := strings.ReplaceAll(s, "\n", " ")
	ret = strings.ReplaceAll(ret, "\u00ad", "")
	ret = strings.ReplaceAll(ret, "\u2019", "'")
	ret = strings.ReplaceAll(ret, "- ", "-")
	ret = strings.ReplaceAll(ret, "SPCL. ATK", "Sp. Atk")
	ret = strings.ReplaceAll(ret, "SPCL. DEF", "Sp. Def")
	ret = strings.ReplaceAll(ret, "SPCL.ATK", "Sp. Atk")
	ret = strings.ReplaceAll(ret, "SPCL.DEF", "Sp. Def")
	ret = strings.ReplaceAll(ret, "SP. ATK", "Sp. Atk")
	ret = strings.ReplaceAll(ret, "SP. DEF", "Sp. Def")
	ret = strings.ReplaceAll(ret, "ATTACK", "Attack")
	ret = strings.ReplaceAll(ret, "DEFENSE", "Defense")
	ret = strings.ReplaceAll(ret, "SPEED", "Speed")
	ret = strings.ReplaceAll(ret, "physi cal", "physical")
	ret = strings.ReplaceAll(ret, "criti cal", "critical")

	return ret
}

func getFlavorText(gen int, lang string, texts []flavorText) string {
	defaultText := getDefaultFlavorText(lang, texts)

	for _, text := range texts {
		id := resolveVersionGroup(text.VersionGroup.Url)
		if gen == id && lang == text.Language.Name {
			return sanitizeString(text.Text)
		}
	}

	return defaultText
}

func getDefaultFlavorText(lang string, texts []flavorText) string {
	for _, text := range texts {
		if lang == text.Language.Name {
			return sanitizeString(text.Text)
		}
	}

	return ""
}

func getGeneration(generation string) int {
	switch generation {
	case "generation-i":
		return 1
	case "generation-ii":
		return 2
	case "generation-iii":
		return 3
	case "generation-iv":
		return 4
	case "generation-v":
		return 5
	case "generation-vi":
		return 6
	case "generation-vii":
		return 7
	case "generation-viii":
		return 8
	default:
		return -1
	}
}

// gets the id at the end of a pokeapi url
func getUrlID(url string) int {
	url_id := strings.Split(url, "/")[6]
	id, err := strconv.Atoi(url_id)
	if err != nil {
		return -1
	}

	return id
}

func createDir(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err = os.MkdirAll(path, 0755); err != nil {
				return err
			}
		}
	}

	return nil
}

func CreateFile(dest, fname string) (*os.File, error) {
	if err := createDir(dest); err != nil {
		return nil, err
	}

	fp := filepath.Join(dest, fname)

	f, err := os.Create(fp)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return f, nil
}

func writeCsvEntry(w *csv.Writer, entry CsvEntry) error {
	if err := w.Write(entry.ToSlice()); err != nil {
		return err
	}

	return nil
}

// func to write APIReceivers to a csv file
func ToCsv(csvFile *os.File, entries []CsvEntry) error {
	if len(entries) == 0 {
		return ErrEmptyCsv
	}

	file, err := os.OpenFile(csvFile.Name(), os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	w := csv.NewWriter(file)
	w.Comma = '|'
	defer w.Flush()

	if err = w.Write(entries[0].GetHeader()); err != nil {
		return err
	}

	for _, entry := range entries {
		if err = writeCsvEntry(w, entry); err != nil {
			return err
		}
	}

	return nil
}

// passes an APIReceiver that will concurrently fetch data from an endpoint
func GetAPIData(recv APIReceiver, limit int, endpoint, lang string) error {
	basicResp, err := getBasicResponse(limit, endpoint)
	if err != nil {
		return err
	}

	recv.Init(basicResp.Count)

	for i := 0; i < basicResp.Count; i++ {
		recv.AddWorker()
		go recv.FetchEntries(basicResp.Results[i].Url, lang, i)
	}

	recv.Wait()
	recv.PostProcess()

	return nil
}

func GenerateCsvs(pr PokemonReceiver, mr MovesReceiver, ar AbilityReceiver) {
	// create csv files
	movesCsv, err := CreateFile("./data/", "moves.csv")
	handleError(err)

	pokemonCsv, err := CreateFile("./data/", "pokemon.csv")
	handleError(err)

	abilityCsv, err := CreateFile("./data/", "ability.csv")
	handleError(err)

	abilityRelCsv, err := CreateFile("./data", "ability-relations.csv")
	handleError(err)

	moveRelCsv, err := CreateFile("./data", "move-relations.csv")
	handleError(err)

	// write csv files
	err = ToCsv(movesCsv, mr.CsvEntries())
	handleError(err)

	err = ToCsv(pokemonCsv, pr.CsvEntries())
	handleError(err)

	err = ToCsv(abilityCsv, ar.CsvEntries())
	handleError(err)

	err = ToCsv(abilityRelCsv, ar.GetCsvRelations())
	handleError(err)

	err = ToCsv(moveRelCsv, pr.GetCsvRelations())
	handleError(err)
}

func handleError(err error) {
	if err != nil {
		log.Fatalf(err.Error())
	}
}

// returns the pokemon's generation it first appeared
// https://bulbapedia.bulbagarden.net/wiki/Generation
func getOriginGeneration(id int) int {
	switch {
	case id <= 151:
		return 1
	case id <= 251:
		return 2
	case id <= 386:
		return 3
	case id <= 493:
		return 4
	case id <= 649:
		return 5
	case id <= 721:
		return 6
	case id <= 809:
		return 7
	case id <= 905:
		return 8
	default:
		return 0
	}
}

func FetchData(limit int, lang string, recv ...APIReceiver) error {
	for _, r := range recv {
		if err := GetAPIData(r, limit, r.GetEndpoint(), lang); err != nil {
			return err
		}
	}

	return nil
}
