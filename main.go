package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Минимальный размер файла для перемещения (в байтах)
const minFileSize int64 = 26463150

// Путь к исходным и целевым папкам
var (
	sourceDirs = []string{
		`e:\FilesNota\572149\1`,
		`e:\FilesNota\572149\2`,
	}
	targetDirs = []string{
		`\\192.168.2.15\5otd\test\1`,
		`\\192.168.2.15\5otd\test\2`,
	}
)

// Имя файла для логов
const logFileName = "transferFiles.log"

func main() {
	// Открытие или создание лог-файла
	logFile, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Ошибка при создании лог-файла: %v\n", err)
		return
	}
	defer logFile.Close()

	// Логгер для записи сообщений
	logger := io.MultiWriter(os.Stdout, logFile)

	log(logger, "[INFO] Программа запущена")

	for i, sourceDir := range sourceDirs {
		targetDir := targetDirs[i]
		log(logger, fmt.Sprintf("Обработка исходной папки: %s", sourceDir))

		err := processDirectory(sourceDir, targetDir, logger)
		if err != nil {
			log(logger, fmt.Sprintf("[ERROR] Ошибка при обработке папки %s: %v", sourceDir, err))
		}
	}

	log(logger, "[INFO] Программа завершена")
}

// processDirectory перемещает файлы из одной папки в другую
func processDirectory(sourceDir, targetDir string, logger io.Writer) error {
	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Проверяем, является ли текущий элемент файлом и подходит ли он по размеру
		if !info.IsDir() && info.Size() >= minFileSize {
			targetPath := filepath.Join(targetDir, info.Name())

			// Перемещение файла
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

// moveFile копирует файл и удаляет исходный
func moveFile(sourcePath, targetPath string) error {
	// Создание целевого файла
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("не удалось открыть исходный файл: %v", err)
	}
	defer sourceFile.Close()

	// Создание файла в целевой папке
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("не удалось создать целевой файл: %v", err)
	}
	defer targetFile.Close()

	// Копирование содержимого
	_, err = io.Copy(targetFile, sourceFile)
	if err != nil {
		return fmt.Errorf("ошибка при копировании содержимого: %v", err)
	}

	// Закрытие файлов для завершения операций
	sourceFile.Close()
	targetFile.Close()

	// Удаление исходного файла
	err = os.Remove(sourcePath)
	if err != nil {
		return fmt.Errorf("не удалось удалить исходный файл: %v", err)
	}

	return nil
}

// log записывает сообщение с отметкой времени
func log(logger io.Writer, message string) {
	timestamp := time.Now().Format("02-01-2006 15:04:05")
	fmt.Fprintf(logger, "%s: %s\n", timestamp, message)
}
