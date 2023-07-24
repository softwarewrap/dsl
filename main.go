package main

import (
	"crypto/tls"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Jeffail/gabs/v2"
	"github.com/JesusIslam/tldr"
	"github.com/blevesearch/bleve/v2"
	"github.com/ghodss/yaml"
	"github.com/k3a/html2text"
	"github.com/xeipuuv/gojsonschema"
)

var (
	message *gabs.Container
	index   bleve.Index
)

func init() {

	// message
	message = gabs.New()
	message.SetP(time.Now(), "metadata.startTimestamp")
	message.SetP("", "body")

	// index
	indexDir := getEnv("INDEX_DIR", "./index")
	os.RemoveAll(indexDir)
	mapping := bleve.NewIndexMapping()
	i, err := bleve.New(indexDir, mapping)
	if err != nil {
		panic(err)
	}
	index = i
}

func main() {

	// loading yaml
	log.Println("loading yaml")
	yamlFile := getEnv("DSL_FILE", "./dsl.yaml")
	yamlContent, err := os.ReadFile(yamlFile)
	if err != nil {
		log.Fatal(err)
	}

	// converting yaml to json
	log.Println("converting yaml to json")
	jsonContent, err := yaml.YAMLToJSON(yamlContent)
	if err != nil {
		log.Fatal(err)
	}

	// loading schema.json file
	log.Println("loading schema.json file")
	schemaFile := getEnv("SCHEMA_FILE", "./schema.json")
	schemaContent, err := os.ReadFile(schemaFile)
	if err != nil {
		log.Fatal(err)
	}

	// validating jsonschema
	log.Println("validating jsonschema")
	schemaLoader := gojsonschema.NewStringLoader(string(schemaContent))
	documentLoader := gojsonschema.NewStringLoader(string(jsonContent))
	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		log.Fatal(err)
	}
	if !result.Valid() {
		log.Fatal("jsonschema validation failed")
	}

	// executing
	log.Println("executing")
	jsonParsed, err := gabs.ParseJSON(jsonContent)
	if err != nil {
		log.Fatal(err)
	}
	for _, task := range jsonParsed.S("tasks").Children() {
		executeTask(task.Data().(map[string]interface{}))
	}

}

func executeTask(m map[string]interface{}) {
	if typ, ok := m["type"]; ok {
		audit(m)
		switch typ {
		case "input":
			taskInput(m)
		case "searchIndex":
			taskSearchIndex(m)
		case "searchQuery":
			taskSearchQuery(m)
		case "summary":
			taskSummary()
		case "log":
			taskLog(m)
		default:
			log.Fatal(typ, " not implemented")
		}
	} else {
		log.Fatal("type is missing")
	}
}

func update(key string, value string) {
	message.SetP(value, key)
}

func get(key string) interface{} {
	jsonParsed, err := gabs.ParseJSON(message.Bytes())
	if err != nil {
		panic(err)
	}

	value := jsonParsed.Path(key).Data()
	return value
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func taskInput(m map[string]interface{}) {
	source := m["source"].(string)

	update("metadata.inputSource", source)

	urlTextFile := getEnv("URL_TEXT_FILE", "./url.txt")

	if _, err := os.Stat(urlTextFile); err == nil {
		fileContent, err := ioutil.ReadFile(urlTextFile)
		if err != nil {
			log.Fatal(err)
		}

		// Convert []byte to string
		text := string(fileContent)
		update("body", text)
	} else {
		fetchHttp(source, "body")
	}

}

func fetchHttp(source string, key string) {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	resp, err := http.Get(source)
	if err != nil {
		log.Fatalln(err)
	}
	//We Read the response body on the line below.
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	//Convert the body to type string
	sb := string(body)
	sb = strings.TrimSpace(sb)
	plain := html2text.HTML2Text(sb)
	plain = strings.Replace(plain, "\r", "", -1)
	plain = strings.Replace(plain, "\n", "", -1)

	update(key, plain)
}

func taskSearchIndex(m map[string]interface{}) {
	index.Index(m["name"].(string), message.String())
}

func taskSearchQuery(m map[string]interface{}) {
	query := bleve.NewQueryStringQuery(m["query"].(string))
	searchRequest := bleve.NewSearchRequest(query)
	searchResult, _ := index.Search(searchRequest)
	log.Println("query: ", query)
	log.Println("result: ", searchResult)
}

func taskSummary() {
	intoSentences := 3
	bag := tldr.New()
	result, _ := bag.Summarize(get("body").(string), intoSentences)
	log.Println("summary: ", result)

	update("summary", strings.Join(result, " "))
}

func taskLog(m map[string]interface{}) {
	log.Println(message.String())
}

func audit(m map[string]interface{}) {
	log.Println("######################")
	log.Println("audit --", m["name"])
}
