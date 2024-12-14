package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Config структура для хранения настроек
type Config struct {
	SourceDirs  []string `json:"source_dirs"`
	TargetDirs  []string `json:"target_dirs"`
	MinFileSize int64    `json:"min_file_size"`
}

const (
	defaultConfigFile = "config.json"
	logDir            = "logs"
	logFileNameFormat = "02-01-2006.log"
	logRetentionDays  = 5
)

func main() {
	// Загрузка или создание конфигурации
	config, err := loadOrCreateConfig(defaultConfigFile)
	if err != nil {
		fmt.Printf("Ошибка при загрузке конфигурации: %v\n", err)
		return
	}

	// Подготовка лог-файла
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

		err := processDirectory(sourceDir, targetDir, config.MinFileSize, logger)
		if err != nil {
			log(logger, fmt.Sprintf("[ERROR] Ошибка при обработке папки %s: %v", sourceDir, err))
		}
	}

	log(logger, "[INFO] Программа завершена")
}

// loadOrCreateConfig загружает настройки из указанного файла или создаёт файл с настройками по умолчанию, если файл отсутствует.
// Если файл конфигурации существует, он считывается и преобразуется в структуру Config.
// Если файл отсутствует, создаётся файл с настройками по умолчанию, записывается на диск и возвращается структура Config с этими настройками.
func loadOrCreateConfig(path string) (*Config, error) {
	// Настройки по умолчанию
	defaultConfig := &Config{
		SourceDirs:  []string{"e:/FilesNota/572149/1", "e:/FilesNota/572149/2"},
		TargetDirs:  []string{"//192.168.2.15/5/test/1", "//192.168.2.15/5/test/2"},
		MinFileSize: 26463150,
	}

	// Попытка открыть файл конфигурации
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		// Если файл не существует, создаётся новый файл с настройками по умолчанию
		file, err := os.Create(path)
		if err != nil {
			return nil, fmt.Errorf("не удалось создать файл конфигурации: %v", err)
		}
		defer file.Close()
		// Сериализация и запись настроек по умолчанию в файл
		prettyJSON, err := json.MarshalIndent(defaultConfig, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("не удалось форматировать настройки: %v", err)
		}
		if _, err := file.Write(prettyJSON); err != nil {
			return nil, fmt.Errorf("не удалось записать настройки: %v", err)
		}
		return defaultConfig, nil
	} else if err != nil {
		// Если ошибка не связана с отсутствием файла, она возвращается
		return nil, fmt.Errorf("ошибка при открытии файла конфигурации: %v", err)
	}
	defer file.Close()

	// Если файл существует, считываем его содержимое
	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("ошибка при чтении файла конфигурации: %v", err)
	}
	return &config, nil
}

// setupLogFile создаёт и открывает лог-файл с именем, соответствующим текущей дате.
// Если директория для логов отсутствует, она создаётся.
func setupLogFile() (*os.File, error) {
	// Создание директории для логов, если она отсутствует
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("не удалось создать директорию для логов: %v", err)
	}
	// Формирование пути к лог-файлу с именем на основе текущей даты
	logFilePath := filepath.Join(logDir, time.Now().Format(logFileNameFormat))
	// Открытие лог-файла в режиме добавления, создания или записи
	return os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
}

// cleanOldLogs удаляет лог-файлы, которые старше определённого количества дней, и пишет об этом в текущий лог-файл.
// Функция сначала получает список файлов в директории логов, затем проверяет дату последнего изменения каждого файла.
// Если файл старше заданного периода (logRetentionDays), он удаляется, а информация об этом записывается в лог.
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

// processDirectory выполняет обход указанной директории, находит файлы, которые соответствуют условиям
// (не являются директориями и имеют размер не менее заданного минимального значения), и перемещает их
// в целевую директорию. Информация об успешных и неудачных операциях записывается в лог.
func processDirectory(sourceDir, targetDir string, minFileSize int64, logger io.Writer) error {
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Size() >= minFileSize {
			targetPath := filepath.Join(targetDir, info.Name())
			err := moveFile(path, targetPath)
			if err != nil {
				log(logger, fmt.Sprintf("[ERROR] Ошибка при перемещении файла %s в %s: %v", path, targetPath, err))
				return err
			}
			log(logger, fmt.Sprintf("Файл %s перемещен в %s", path, targetPath))
		}
		return nil
	})
}

// moveFile копирует файл из исходного пути в целевой, а затем удаляет исходный файл
// только если копирование прошло успешно.
func moveFile(sourcePath, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("не удалось открыть исходный файл: %v", err)
	}
	defer sourceFile.Close()

	// Проверяем, существует ли целевой файл, чтобы избежать перезаписи
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

	// Копирование содержимого файла
	_, err = io.Copy(targetFile, sourceFile)
	if err != nil {
		return fmt.Errorf("ошибка при копировании содержимого: %v", err)
	}

	// Закрываем файлы перед удалением, чтобы освободить ресурсы
	sourceFile.Close()
	targetFile.Close()

	// Удаляем исходный файл только если копирование прошло успешно
	if err := os.Remove(sourcePath); err != nil {
		return fmt.Errorf("не удалось удалить исходный файл после копирования: %v", err)
	}

	return nil
}

// log записывает сообщение в указанный логгер с текущей датой и временем.
func log(logger io.Writer, message string) {
	timestamp := time.Now().Format("02-01-2006 15:04:05")
	fmt.Fprintf(logger, "%s: %s\n", timestamp, message)
}
