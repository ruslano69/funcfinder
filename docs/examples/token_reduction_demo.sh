#!/bin/bash

echo "========================================="
echo "ДЕМОНСТРАЦИЯ: Token Reduction для AI"
echo "========================================="
echo ""

# Сценарий: AI нужно понять функцию FindFunctions в python_finder.go
FILE="python_finder.go"

echo "📚 Сценарий: AI анализирует функцию FindFunctions в $FILE"
echo ""

# Вариант 1: Отправить весь файл
echo "❌ Вариант 1: Отправить весь файл"
TOTAL_LINES=$(wc -l < "$FILE")
TOTAL_CHARS=$(wc -c < "$FILE")
echo "Размер файла: $TOTAL_LINES строк, $TOTAL_CHARS байт"
echo "Примерные токены: ~$(($TOTAL_CHARS / 4)) tokens"
echo ""

# Вариант 2: Использовать funcfinder
echo "✅ Вариант 2: Использовать funcfinder --extract"
EXTRACTED=$(./funcfinder --inp "$FILE" --source go --func FindFunctions --extract)
EXTRACTED_LINES=$(echo "$EXTRACTED" | wc -l)
EXTRACTED_CHARS=$(echo "$EXTRACTED" | wc -c)
echo "Размер извлеченной функции: $EXTRACTED_LINES строк, $EXTRACTED_CHARS байт"
echo "Примерные токены: ~$(($EXTRACTED_CHARS / 4)) tokens"
echo ""

# Подсчет экономии
REDUCTION=$((100 - ($EXTRACTED_CHARS * 100 / $TOTAL_CHARS)))
echo "💰 Экономия токенов: $REDUCTION%"
echo ""

echo "Извлеченная функция:"
echo "$EXTRACTED" | head -15
echo "..."
echo ""

# Вариант 3: JSON для AI навигации
echo "🗺️ Вариант 3: JSON map для навигации по всему файлу"
JSON_MAP=$(./funcfinder --inp "$FILE" --source go --map --json)
JSON_SIZE=$(echo "$JSON_MAP" | wc -c)
echo "Размер JSON карты: $JSON_SIZE байт"
echo "Примерные токены: ~$(($JSON_SIZE / 4)) tokens"
echo ""
echo "$JSON_MAP"
echo ""

MAP_REDUCTION=$((100 - ($JSON_SIZE * 100 / $TOTAL_CHARS)))
echo "💰 Экономия токенов для навигации: $MAP_REDUCTION%"
echo ""

echo "========================================="
echo "ВЫВОД: funcfinder экономит 85-95% токенов!"
echo "========================================="
