package downloader

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

var mutex sync.Mutex

type emote struct {
	id              string
	name            string
	urlsByImageType map[string]map[string]string
}

type emoteSet struct {
	id       string
	name     string
	capacity float64
	emotes   []*emote
}

type emoteDownloader struct {
	user                     map[string]any
	emoteSets                []*emoteSet
	currentDownloadIndicator string
	percentage               float64
}

func (e *emoteSet) String() string {
	return fmt.Sprintf("%s [%d Emotes]", e.name, len(e.emotes))
}

func fetchUser(userId string) (map[string]any, error) {

	userUrl := fmt.Sprintf("https://7tv.io/v3/users/%s", "01H3FRQXT00007Y62Y79WSW8PM")

	userResp, uErr := http.Get(userUrl)
	if uErr != nil {
		return nil, fmt.Errorf("error connecting to api: %v", uErr)
	}
	defer userResp.Body.Close()

	if userResp.StatusCode != 200 {
		return nil, fmt.Errorf("user not found. Error Code: %s", userResp.Status)
	}

	body, rErr := io.ReadAll(userResp.Body)
	if rErr != nil {
		return nil, fmt.Errorf("error while reading response: %v", rErr)
	}

	var user map[string]any

	jErr := json.Unmarshal(body, &user)
	if jErr != nil {
		return nil, fmt.Errorf("can't unmarshal response body: %v", jErr)
	}

	//emoteSets := fetchEmoteSetsOfUser(user["emote_sets"].([]any))

	// select emote sets
	/*selectedSets := selector(emoteSets, "emote set")

	// select image type
	availableImageTypes := []string{"webp", "avif", "png", "gif"}
	selectedImageTypes := selector(availableImageTypes, "image type")

	// select image sizes
	availableImageSizes := []string{"1x", "2x", "3x", "4x"}
	selectedSizes := selector(availableImageSizes, "image size")

	fmt.Println(selectedImageTypes, selectedSizes)*/

	//startTime := time.Now()
	//downloadAndWriteEmoteSets(selectedSets, selectedImageTypes, selectedSizes, username)
	//endTime := time.Now()

	//took := endTime.Sub(startTime)
	//fmt.Printf("took %.2f seconds.\n", float64(took.Milliseconds())/1000)

	return user, nil
}

func downloadAndWriteEmoteSets(model *model, emoteSets []*emoteSet, selectedTypes []string, selectedSizes []string, username string) {
	for _, set := range emoteSets {
		emotePath := fmt.Sprintf("emotes/%s/%s", username, set.name)
		if err := os.MkdirAll(emotePath, os.ModePerm); err != nil {
			*model.stateHeader = fmt.Sprintf("Failed to create directory for emote set: %s\n Error: %s", set.name, err)
		}

		*model.stateHeader = fmt.Sprintf("Emote Set: %s: ", set.name)
		percentageIncrease := 1 / float64(len(set.emotes))

		// download emotes
		group := new(errgroup.Group)
		group.SetLimit(runtime.NumCPU())
		for i := range set.emotes {
			group.Go(func() error {
				downloadEmote(model, emotePath, set.emotes[i], selectedTypes, selectedSizes, percentageIncrease)
				return nil
			})
		}

		err := group.Wait()
		if err != nil {
			model.emoteDownloader.currentDownloadIndicator = err.Error()
		}

		model.emoteDownloader.currentDownloadIndicator = fmt.Sprintf("Successfully downloaded %d emotes from the %s emote set of %s.", len(set.emotes), set.name, username)
	}
}

func downloadEmote(model *model, emotePath string, emote *emote, selectedTypes []string, selectedSizes []string, percentageIncrease float64) {
	wg := sync.WaitGroup{}
	for _, fileType := range selectedTypes {
		fileType = strings.ToUpper(fileType)
		if _, exists := emote.urlsByImageType[fileType]; exists {
			for _, size := range selectedSizes {
				if _, exists := emote.urlsByImageType[fileType][size]; exists {
					wg.Add(1)
					go func() {
						defer wg.Done()

						startTime := time.Now()
						url := emote.urlsByImageType[fileType][size]
						// emotes/username/emoteset/GIF4x/
						directoryPath := emotePath + "/" + fileType + size

						fileName := emote.name + "." + strings.ToLower(fileType)
						// emotes/username/emoteset/GIF4x/fileName.gif
						filePath := directoryPath + "/" + fileName

						if _, err := os.Stat(filePath); err == nil {
							mutex.Lock()
							model.emoteDownloader.currentDownloadIndicator = fmt.Sprintf("%s already exists - skipping\n", filePath)
							model.emoteDownloader.percentage += percentageIncrease
							mutex.Unlock()
							return
						}

						resp, err := http.Get(url)
						if err != nil {
							fmt.Printf("Failed to get emote %s %v\n", emote.name, err)
							return
						}
						defer resp.Body.Close()

						if resp.StatusCode != 200 {
							fmt.Printf("Failed to get emote %s Error: %s\n", emote.name, resp.Status)
						}

						// write to file
						if err := os.MkdirAll(directoryPath, os.ModePerm); err != nil {
							fmt.Printf("Failed to create directory %s for emote: %s\n Error: %s", directoryPath, emote.name, err)
						}

						if file, err := os.Create(filePath); err == nil {
							defer file.Close()
							_, err := io.Copy(file, resp.Body)
							if err != nil {
								fmt.Printf("Failed to write emote to file %s\n", fileName)
							}

							endTime := time.Now()
							took := endTime.Sub(startTime)

							mutex.Lock()
							model.emoteDownloader.percentage += percentageIncrease
							model.emoteDownloader.currentDownloadIndicator = fmt.Sprintf("Downloaded %s - took %.2f seconds", emote.name, float64(took.Milliseconds())/1000)
							mutex.Unlock()
						} else {
							fmt.Printf("Failed to create file for %s\n Error: %s\n", fileName, err)
						}

					}()
				}
			}
		}
	}
	wg.Wait()
}

/*
	func selector[T any](toSelect []T, selectionName string) []T {
		fmt.Println("----------------------------------------------------------------------------------------------------------------")
		fmt.Printf("Please select which %ss you want to download. Seperate each number with a comma. Example: 1,3,4\n", selectionName)
		fmt.Printf("If you want to download every %s, just type 'a' or 'all'\n", selectionName)

		for i, val := range toSelect {
			fmt.Printf("(%d) %v\n", i+1, val)
		}

		selected := make([]T, 0)
		for i := 0; ; i++ {
			var selection string
			fmt.Scanln(&selection)
			selection = strings.TrimSpace(selection)

			usedNums := make([]int, 0)
			if selection == "a" || selection == "all" {
				selected = toSelect
			} else {
				isValid := true
				for _, text := range strings.Split(selection, ",") {
					if num, err := strconv.Atoi(text); err != nil {
						// clear lines
						if i > 0 {
							fmt.Printf("\033[1A\033[K")
						}
						fmt.Printf("\033[1A\033[K")

						fmt.Println("Only input numbers!")
						isValid = false
						break
						//return selector(toSelect, selectionName, false)
					} else {
						num := int(num)
						if num <= 0 || num > len(toSelect) {
							fmt.Println("Only input numbers that are listed!")
							isValid = false
							break
						}
						// skip duplicates
						if slices.Contains(usedNums, num) {
							continue
						}

						selected = append(selected, toSelect[num-1])
						usedNums = append(usedNums, num)
					}
				}
				if isValid {
					break
				}
			}
		}

		return selected

}
*/
func fetchEmoteSetsOfUser(emoteSets []any) []*emoteSet {
	emoteSetIds := make([]string, len(emoteSets))

	for i, set := range emoteSets {
		set := set.(map[string]any)
		emoteSetIds[i] = set["id"].(string)
	}

	// fetch emote sets in parallel
	setChannels := make([]<-chan *emoteSet, 0)
	for _, id := range emoteSetIds {
		setChannel := make(chan *emoteSet)
		go fetchEmoteSet(id, setChannel)
		setChannels = append(setChannels, setChannel)
	}

	sets := make([]*emoteSet, 0)
	for _, channel := range setChannels {
		if set := <-channel; set != nil {
			set := *set

			sets = append(sets, &set)
		}
	}

	return sets
}

func fetchEmoteSet(id string, setChannel chan *emoteSet) {
	defer close(setChannel)
	apiUrl := fmt.Sprintf("https://7tv.io/v3/emote-sets/%s", id)

	setResp, err := http.Get(apiUrl)
	if err != nil {
		fmt.Println("error while fetching emoteset:", err)
		return
	}
	defer setResp.Body.Close()

	if setResp.StatusCode != 200 {
		fmt.Println("Emote set not found")
		return
	}

	body, err := io.ReadAll(setResp.Body)
	if err != nil {
		fmt.Println("error while reading emoteset response:", err)
		return
	}

	var set map[string]any
	if err := json.Unmarshal(body, &set); err != nil {
		fmt.Println("error while unmarshalling body:", err)
		return
	}

	respEmotes := set["emotes"]
	if respEmotes == nil {
		return
	}

	// fetch emotes in parallel
	emoteChannels := make([]<-chan *emote, 0)
	for _, respEmote := range respEmotes.([]any) {
		respEmote := respEmote.(map[string]any)
		data := respEmote["data"].(map[string]any)

		emoteChannel := make(chan *emote)
		go createEmoteFromData(data, emoteChannel)
		emoteChannels = append(emoteChannels, emoteChannel)
	}

	emotes := make([]*emote, 0)
	for _, channel := range emoteChannels {
		if emote := <-channel; emote != nil {
			emotes = append(emotes, emote)
		}
	}

	newEmoteSet := emoteSet{
		id:       set["id"].(string),
		name:     strings.Trim(set["name"].(string), " "),
		capacity: set["capacity"].(float64),
		emotes:   emotes,
	}

	setChannel <- &newEmoteSet
}

func createEmoteFromData(emoteData map[string]any, emoteChannel chan<- *emote) {
	defer close(emoteChannel)
	emote := emote{
		id:   emoteData["id"].(string),
		name: emoteData["name"].(string),
	}
	host := emoteData["host"].(map[string]any)
	baseUrl := "https:" + host["url"].(string)

	urlsByImageType := make(map[string]map[string]string)
	files := host["files"].([]any)

	for _, file := range files {
		file := file.(map[string]any)

		format := file["format"].(string)
		if _, exists := urlsByImageType[format]; !exists {
			urlsByImageType[format] = make(map[string]string, 0)
		}

		name := file["name"].(string)
		fileUrl := baseUrl + "/" + name
		size := strings.Split(name, ".")[0]
		urlsByImageType[format][size] = fileUrl
	}

	emote.urlsByImageType = urlsByImageType

	emoteChannel <- &emote
}
