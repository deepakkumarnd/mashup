package main

// https://www.digitalocean.com/community/tutorials/how-to-make-an-http-server-in-go

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

const DEFAULT_BUILD_QUEUE_SIZE = 3

type TaskDefinition struct {
	Id             string    `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	GitUrl         string    `json:"gitUrl"`
	Branch         string    `json:"branch`
	DockerHubUrl   string    `json:"dockerHubUrl"`
	DockerRepoName string    `json:"dockerRepoName"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type Build struct {
	taskDefinition TaskDefinition
	queue          chan string
}

func (b *Build) pipeline() {
	for id := range b.queue {
		log.Println("Building ", id)
		time.Sleep(1 * time.Minute)
	}
}

func (b *Build) Enqueue() bool {
	if len(b.queue) == cap(b.queue) {
		return false
	} else {
		b.queue <- b.taskDefinition.Id
		return true
	}
}

func (d *TaskDefinition) toString() string {
	data, err := json.Marshal(d)

	if err != nil {
		data = []byte{}
	}

	return string(data)
}

func (d *TaskDefinition) saveToDisk() {
	if _, err := os.Stat("db"); os.IsNotExist(err) {
		os.Mkdir("db", 0777)
	}

	err := os.WriteFile("db/"+d.Id+".json", []byte(d.toString()), 0777)

	if err != nil {
		log.Printf("Error writing definition fto ile %s\n", err)
	}
}

func loadAllTaskDefinitions() map[string]TaskDefinition {

	mapping := make(map[string]TaskDefinition)

	if _, err := os.Stat("db"); os.IsNotExist(err) {
		return mapping
	}

	files, err := os.ReadDir("db")

	if err != nil {
		panic("Error in reading directory 'db'")
	}

	for _, file := range files {
		filename := "db/" + file.Name()
		data, err := os.ReadFile(filename)

		if err != nil {
			panic("Error in reading file " + filename)
		}

		taskDefinition := TaskDefinition{}

		err1 := json.Unmarshal(data, &taskDefinition)

		if err1 != nil {
			panic("Error in parsing file " + file.Name())
		}

		mapping[taskDefinition.Id] = taskDefinition
	}

	return mapping
}

func loadAllBuilds(taskDefinitions map[string]TaskDefinition) {
	for _, definition := range taskDefinitions {
		initBuild(definition)
	}
}

var TaskDefinitionsMap map[string]TaskDefinition

var BuildMap map[string]Build = make(map[string]Build)

func Init() {
	TaskDefinitionsMap = loadAllTaskDefinitions()
	loadAllBuilds(TaskDefinitionsMap)
}

func initBuild(taskDefinition TaskDefinition) Build {
	build := Build{taskDefinition: taskDefinition, queue: make(chan string, DEFAULT_BUILD_QUEUE_SIZE)}
	oldBuild, ok := BuildMap[taskDefinition.Id]

	if ok {
		close(oldBuild.queue)
		// Drain channel ?
	}

	BuildMap[taskDefinition.Id] = build
	go build.pipeline()

	return build
}

// var BuildMap map[string]Build = loadAllBuilds()

func NewUUID() string {
	return uuid.Must(uuid.NewRandom()).String()
}

// GET /
func getRoot(w http.ResponseWriter, request *http.Request) {
	log.Println("Got request /")
	io.WriteString(w, "OK")
}

// GET /up
func getUp(w http.ResponseWriter, request *http.Request) {
	log.Println("Got request /up")
	io.WriteString(w, "OK")
}

// POST /create
func createBuild(w http.ResponseWriter, request *http.Request) {

	log.Println("Got request /create-build")

	type CreateBuildRequest struct {
		Name           string `json:"name" validate:"required,min=4,max=255"`
		Description    string `json:"description" validate:"required,min=8,max=255"`
		GitUrl         string `json:"git-url" "required,min=4,max=255"`
		Branch         string `json:"branch" "required,min=2,max=64"`
		DockerHubUrl   string `json:"dockerHubUrl" "required,min=4,max=255"`
		DockerRepoName string `json:"dockerRepoName" "required,min=2,max=64"`
	}

	var createRequest CreateBuildRequest

	var buildTaskDefinition = func(requestData CreateBuildRequest) TaskDefinition {
		return TaskDefinition{
			Id:             NewUUID(),
			Name:           requestData.Name,
			Description:    requestData.Description,
			GitUrl:         requestData.GitUrl,
			Branch:         requestData.Branch,
			DockerHubUrl:   requestData.DockerHubUrl,
			DockerRepoName: requestData.DockerRepoName,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
	}

	body, err := io.ReadAll(request.Body)

	var responseStr string

	if err != nil {
		responseStr = fmt.Sprintf("Could not read request body %s \n", err)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Println(responseStr)
	} else {
		err := json.Unmarshal(body, &createRequest)

		if err != nil {
			responseStr = fmt.Sprintln("Bad request")
			w.WriteHeader(http.StatusBadRequest)
		} else {
			log.Println(string(body))
			taskDefinition := buildTaskDefinition(createRequest)
			TaskDefinitionsMap[taskDefinition.Id] = taskDefinition
			initBuild(taskDefinition)
			responseStr = taskDefinition.toString()
			taskDefinition.saveToDisk()
			w.WriteHeader(http.StatusCreated)
		}
	}

	io.WriteString(w, responseStr)
}

func getBuild(w http.ResponseWriter, request *http.Request) {
	log.Println("Got request /get-build")
	request.URL.Query().Has("id")
	id := request.URL.Query().Get("id")
	taskDefinition, ok := TaskDefinitionsMap[id]

	if ok {
		io.WriteString(w, taskDefinition.toString())
	} else {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, "")
	}
}

func listAll(w http.ResponseWriter, request *http.Request) {
	log.Println("Got request /list-all-builds")

	type GetAllResponse struct {
		Total       int              `json:"count"`
		Definitions []TaskDefinition `json:"definitions"`
	}

	count := len(TaskDefinitionsMap)

	list := make([]TaskDefinition, 0, count)

	fmt.Println("Initial length of list is", len(list))

	for _, value := range TaskDefinitionsMap {
		list = append(list, value)
	}

	responseStr, err := json.Marshal(GetAllResponse{Total: count, Definitions: list})

	if err != nil {
		io.WriteString(w, "")
	} else {
		io.WriteString(w, string(string(responseStr)))
	}
}

func updateBuild(w http.ResponseWriter, request *http.Request) {
	log.Println("Got request /update-build")

	type UpdateBuildRequest struct {
		Name           string `json:"name" validate:"required,min=4,max=255"`
		Description    string `json:"description" validate:"required,min=8,max=255"`
		GitUrl         string `json:"git-url" "required,min=4,max=255"`
		Branch         string `json:"branch" "required,min=2,max=64"`
		DockerHubUrl   string `json:"dockerHubUrl" "required,min=4,max=255"`
		DockerRepoName string `json:"dockerRepoName" "required,min=2,max=64"`
	}

	var updateRequest UpdateBuildRequest

	var responseStr string

	request.URL.Query().Has("id")
	id := request.URL.Query().Get("id")

	taskDefinition, ok := TaskDefinitionsMap[id]

	var updateTaskDefinition = func() {
		taskDefinition.Name = updateRequest.Name
		taskDefinition.Description = updateRequest.Description
		taskDefinition.GitUrl = updateRequest.GitUrl
		taskDefinition.DockerHubUrl = updateRequest.DockerHubUrl
		taskDefinition.DockerRepoName = updateRequest.DockerRepoName
		taskDefinition.UpdatedAt = time.Now()
	}

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, "Task definition not found")
		return
	}

	body, err := io.ReadAll(request.Body)

	if err != nil {
		responseStr = fmt.Sprintf("Could not read request body %s \n", err)
		fmt.Println(responseStr)
		w.WriteHeader(http.StatusBadRequest)
	} else {
		err := json.Unmarshal(body, &updateRequest)

		if err != nil {
			responseStr = fmt.Sprintln("Bad request")
			w.WriteHeader(http.StatusBadRequest)
		} else {
			log.Println(string(body))
			updateTaskDefinition()
			TaskDefinitionsMap[taskDefinition.Id] = taskDefinition
			initBuild(taskDefinition)
			responseStr = taskDefinition.toString()
			taskDefinition.saveToDisk()
			w.WriteHeader(http.StatusCreated)
		}
	}

	io.WriteString(w, responseStr)
}

func startBuild(w http.ResponseWriter, request *http.Request) {
	request.URL.Query().Has("id")
	id := request.URL.Query().Get("id")

	log.Printf("Got request /start-build?id=%s\n", id)

	if len(strings.TrimSpace(id)) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "Invalid build identifier")
		return
	}

	build, ok := BuildMap[id]

	if !ok {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, "Not found task definition matching id")
		return
	}

	if build.Enqueue() {
		io.WriteString(w, "New build enqueued")
	} else {
		io.WriteString(w, "Rejected, build queue full")
	}

}

func main() {
	log.Println("Starting builder")
	Init()

	mux := http.NewServeMux()
	mux.HandleFunc("/", getRoot)
	mux.HandleFunc("/up", getUp)
	mux.HandleFunc("/create-build", createBuild)
	mux.HandleFunc("/get-build", getBuild)
	mux.HandleFunc("/list-all-builds", listAll)
	mux.HandleFunc("/update-build", updateBuild)
	mux.HandleFunc("/start-build", startBuild)

	log.Println("Starting server on port 8080 ...")

	err := http.ListenAndServe(":8080", mux)

	if errors.Is(err, http.ErrServerClosed) {
		fmt.Println("Server closed")
	} else if err != nil {
		fmt.Printf("Error starting server %s\n", err)
	}
}
