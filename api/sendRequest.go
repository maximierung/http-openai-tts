package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/maximierung/http-openai-tts/database"
	"github.com/maximierung/http-openai-tts/structs"
	"github.com/maximierung/http-openai-tts/utils"
	"go.mongodb.org/mongo-driver/mongo"
)

func SendRequest(client *mongo.Client, w http.ResponseWriter, r *http.Request) {
	config, _ := utils.LoadConfig()
	apiKey := config.ApiKey
	url := "https://api.openai.com/v1/audio/speech"

	key := r.Header.Get("KEY")
	if config.NeedKey {
		if !database.CheckKey(client, config.DBName, key) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("{\"status\": \"Key error\",\"message\": \"The key provided in the 'KEY' header doesn't exist.\"}"))
			return
		}
		database.AddCall(client, config.DBName, key)
	}

	model := strings.ToLower(r.FormValue("model"))
	voice := strings.ToLower(r.FormValue("voice"))
	format := strings.ToLower(r.FormValue("format"))
	text := r.FormValue("text")
	unparsedSpeed := r.FormValue("speed")
	speed, err := strconv.ParseFloat(unparsedSpeed, 64)

	if !utils.CheckModel(model) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("{\"status\": \"Input error\",\"message\": \"The provided model doesn't exist.\"}"))
		return
	}

	if !utils.CheckVoice(voice) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("{\"status\": \"Input error\",\"message\": \"The provided voice doesn't exist.\"}"))
		return
	}

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("{\"status\": \"Input error\",\"message\": \"An error occurred while trying to parse the 'speed' value.\"}"))
		return
	}

	if !utils.CheckSpeed(speed) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("{\"status\": \"Input error\",\"message\": \"The provided 'speed' value is out of range.\"}"))
		return
	}

	if !utils.CheckFormat(format) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("{\"status\": \"Input error\",\"message\": \"The provided 'format' value is not supported.\"}"))
		return
	}

	if !utils.CheckText(len(text)) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("{\"status\": \"Input error\",\"message\": \"The provided 'text' value is out of range.\"}"))
		return
	}

	requestBody := structs.Request{
		Model:          model,
		Input:          text,
		Voice:          voice,
		ResponseFormat: format,
		Speed:          speed,
	}

	jsonRequestBody, err := json.Marshal(requestBody)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("{\"status\": \"Internal error\",\"message\": \"An error occurred while encoding JSON.\",\"error\": \"%v\"}", err)))
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonRequestBody))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("{\"status\": \"Internal error\",\"message\": \"An error occurred while creating the POST request.\",\"error\": \"%v\"}", err)))
		return
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("{\"status\": \"Internal error\",\"message\": \"An error occurred while sending the POST request.\",\"error\": \"%v\"}", err)))
		return
	}

	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("{\"status\": \"Internal error\",\"message\": \"An error occurred while reading the response body.\",\"error\": \"%v\"}", err)))
		return
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("API Error: Status Code %d\n", resp.StatusCode)
		w.WriteHeader(http.StatusFailedDependency)
		w.Write([]byte(fmt.Sprintf("{\"status\": \"OpenAI API error\",\"message\": \"The HTTP response status is not 200.\",\"response\": \"%s\"}", responseBody)))
		return
	}

	// delete ASAP!!!!!!!!!
	contentType := resp.Header.Get("Content-Type")
	fmt.Println("Content-Type:", contentType)

	filename := uuid.New().String()

	file, err := os.Create(fmt.Sprintf("./output/%s.%s", filename, format))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("{\"status\": \"Internal error\",\"message\": \"An error occurred while creating the local file.\",\"error\": \"%v\"}", err)))
		return
	}
	defer file.Close()

	_, err = file.Write(responseBody)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("{\"status\": \"Internal error\",\"message\": \"An error occurred while writing to the local file.\",\"error\": \"%v\"}", err)))
		return
	} else {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("{\"status\": \"OK\",\"message\": \"TTS generation successfull.\",\"link\": \"%s://%s/%s.%s\"}", config.TransferProtocol, r.Host, filename, format)))
		fmt.Printf("Audio saved to %s.%s\n", filename, format)
		return
	}
}