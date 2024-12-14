package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config структура для хранения настроек
type Config struct {
	SourceDirs   []string `json:"source_dirs"`
	TargetDirs   []string `json:"target_dirs"`
	MinFileSize  int64    `json:"min_file_size"`
	DateTemplate string   `json:"date_template"`
}

const (
	defaultConfigFile = "config.json"
	logDir            = "logs"
	logFileNameFormat = "02-01-2006.log"
	logRetentionDays  = 5
)

func main() {
	config, err := loadOrCreateConfig(defaultConfigFile)
	if err != nil {
		fmt.Printf("Ошибка при загрузке конфигурации: %v\n", err)
		return
	}

	logFile, err := setupLogFile()
	if err != nil {
		fmt.Printf("Ошибка при создании лог-файла: %v\n", err)
		return
	}
	defer logFile.Close()
	logger := io.MultiWriter(os.Stdout, logFile)

	log(logger, "[INFO] Программа запущена")
	cleanOldLogs(logger)

	for i, sourceDir := range config.SourceDirs {
		targetDir := config.TargetDirs[i]
		log(logger, fmt.Sprintf("Обработка исходной папки: %s", sourceDir))

		err := processDirectory(sourceDir, targetDir, config.MinFileSize, config.DateTemplate, logger)
		if err != nil {
			log(logger, fmt.Sprintf("[ERROR] Ошибка при обработке папки %s: %v", sourceDir, err))
		}
	}

	log(logger, "[INFO] Программа завершена")
}

func extractDate(fileName, template string) (string, string, string, error) {
	var yearPart, monthPart, dayPart strings.Builder
	fileIndex := 0

	for _, ch := range template {
		templateChar := string(ch)
		switch templateChar {
		case "Г":
			yearPart.WriteByte(fileName[fileIndex])
		case "М":
			monthPart.WriteByte(fileName[fileIndex])
		case "Д":
			dayPart.WriteByte(fileName[fileIndex])
		case "?":
			// Пропуск символов
		default:
			return "", "", "", fmt.Errorf("шаблон содержит недопустимый символ: %s", templateChar)
		}
		fileIndex++
	}

	if _, err := strconv.Atoi(yearPart.String()); err != nil {
		return "", "", "", fmt.Errorf("не удалось разобрать год: %v", err)
	}
	if _, err := strconv.Atoi(monthPart.String()); err != nil {
		return "", "", "", fmt.Errorf("не удалось разобрать месяц: %v", err)
	}
	if _, err := strconv.Atoi(dayPart.String()); err != nil {
		return "", "", "", fmt.Errorf("не удалось разобрать день: %v", err)
	}

	return yearPart.String(), monthPart.String(), dayPart.String(), nil
}

func loadOrCreateConfig(path string) (*Config, error) {
	defaultConfig := &Config{
		SourceDirs:   []string{"e:/FilesNota/572149/1", "e:/FilesNota/572149/2"},
		TargetDirs:   []string{"//192.168.2.15/5/test/1", "//192.168.2.15/5/test/2"},
		MinFileSize:  26463150,
		DateTemplate: "??ГГГГ?ММ?ДД",
	}

	file, err := os.Open(path)
	if os.IsNotExist(err) {
		file, err := os.Create(path)
		if err != nil {
			return nil, fmt.Errorf("не удалось создать файл конфигурации: %v", err)
		}
		defer file.Close()
		prettyJSON, err := json.MarshalIndent(defaultConfig, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("не удалось форматировать настройки: %v", err)
		}
		if _, err := file.Write(prettyJSON); err != nil {
			return nil, fmt.Errorf("не удалось записать настройки: %v", err)
		}
		return defaultConfig, nil
	} else if err != nil {
		return nil, fmt.Errorf("ошибка при открытии файла конфигурации: %v", err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("ошибка при чтении файла конфигурации: %v", err)
	}
	return &config, nil
}

func setupLogFile() (*os.File, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("не удалось создать директорию для логов: %v", err)
	}
	logFilePath := filepath.Join(logDir, time.Now().Format(logFileNameFormat))
	return os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}

func cleanOldLogs(logger io.Writer) {
	files, err := os.ReadDir(logDir)
	if err != nil {
		log(logger, fmt.Sprintf("[ERROR] Не удалось прочитать директорию логов: %v", err))
		return
	}

	cutoff := time.Now().AddDate(0, 0, -logRetentionDays)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		filePath := filepath.Join(logDir, file.Name())
		info, err := os.Stat(filePath)
		if err != nil {
			log(logger, fmt.Sprintf("[ERROR] Не удалось получить информацию о файле %s: %v", file.Name(), err))
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filePath); err != nil {
				log(logger, fmt.Sprintf("[ERROR] Не удалось удалить старый лог-файл %s: %v", file.Name(), err))
			} else {
				log(logger, fmt.Sprintf("Удален старый лог-файл: %s", file.Name()))
			}
		}
	}
}

func processDirectory(sourceDir, targetDir string, minFileSize int64, template string, logger io.Writer) error {
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Size() >= minFileSize {
			year, month, day, err := extractDate(info.Name(), template)
			if err != nil {
				log(logger, fmt.Sprintf("[ERROR] Ошибка извлечения даты из файла %s: %v", info.Name(), err))
				return err
			}

			dateFolder := fmt.Sprintf("%s-%s-%s", day, month, year)
			dateFolderPath := filepath.Join(targetDir, dateFolder)

			if _, err := os.Stat(dateFolderPath); os.IsNotExist(err) {
				if err := os.Mkdir(dateFolderPath, 0755); err != nil {
					log(logger, fmt.Sprintf("[ERROR] Не удалось создать папку %s: %v", dateFolderPath, err))
					return err
				}
				log(logger, fmt.Sprintf("Создана папка: %s", dateFolderPath))
			}

			targetPath := filepath.Join(dateFolderPath, info.Name())
			if err := moveFile(path, targetPath); err != nil {
				log(logger, fmt.Sprintf("[ERROR] Ошибка при перемещении файла %s в %s: %v", path, targetPath, err))
				return err
			}

			log(logger, fmt.Sprintf("Файл %s перемещен в %s", path, targetPath))
		}
		return nil
	})
}

func moveFile(sourcePath, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("не удалось открыть исходный файл: %v", err)
	}
	defer sourceFile.Close()

	if _, err := os.Stat(targetPath); err == nil {
		return fmt.Errorf("целевой файл уже существует: %s", targetPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("ошибка при проверке целевого файла: %v", err)
	}

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("не удалось создать целевой файл: %v", err)
	}
	defer targetFile.Close()

	if _, err = io.Copy(targetFile, sourceFile); err != nil {
		return fmt.Errorf("ошибка при копировании содержимого: %v", err)
	}

	sourceFile.Close()
	targetFile.Close()

	if err := os.Remove(sourcePath); err != nil {
		return fmt.Errorf("не удалось удалить исходный файл после копирования: %v", err)
	}

	return nil
}

func log(logger io.Writer, message string) {
	timestamp := time.Now().Format("02-01-2006 15:04:05")
	fmt.Fprintf(logger, "%s: %s\n", timestamp, message)
}
