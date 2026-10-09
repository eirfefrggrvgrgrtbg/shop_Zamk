package marketing

import (
	"fmt"
	"strconv"
	"strings"
)

type ExportColumnDef struct {
	Key    string
	Header string
}

func mapExportHeader(colKey string) string {
	switch colKey {
	case "day":
		return "День"
	case "source":
		return "Источник"
	case "campaignName", "campaign":
		return "Кампания"
	case "productName", "product":
		return "Товар"
	case "designerName", "designer":
		return "Дизайнер"
	case "categoryName", "category":
		return "Категория"
	case "sessions":
		return "Сессии"
	case "orders":
		return "Заказы"
	case "sold_units":
		return "Продано единиц"
	case "revenue":
		return "Выручка"
	case "product_views":
		return "Просмотры товара"
	case "favorites":
		return "Добавления в избранное"
	case "add_to_cart":
		return "Добавления в корзину"
	case "conversion":
		return "Конверсия, %"
	case "returns":
		return "Возвраты"
	case "returned_units":
		return "Возвращено единиц"
	default:
		return colKey
	}
}

func BuildExportColumns(queryCols []QueryColumn) []ExportColumnDef {
	var out []ExportColumnDef
	seenSource := false

	for _, col := range queryCols {
		if col.Key == "sourceKey" || col.Key == "sourceKind" {
			if !seenSource {
				seenSource = true
				out = append(out, ExportColumnDef{Key: "source", Header: "Источник"})
			}
			continue
		}
		if col.Key == "source" {
			if !seenSource {
				seenSource = true
				out = append(out, ExportColumnDef{Key: "source", Header: "Источник"})
			}
			continue
		}

		out = append(out, ExportColumnDef{
			Key:    col.Key,
			Header: mapExportHeader(col.Key),
		})
	}
	return out
}

func formatSourceVal(row map[string]interface{}) string {
	sKind, _ := row["sourceKind"].(string)
	sKey, _ := row["sourceKey"].(string)
	sVal, _ := row["source"].(string)

	if sKind == "direct" || sKey == "direct" || sVal == "direct" {
		return "Прямой заход"
	}
	if sKind == "unattributed" || (sKey == "" && sVal == "") {
		return "Неизвестный источник"
	}
	if sVal != "" {
		return sVal
	}
	if sKey != "" {
		return sKey
	}
	return "Неизвестный источник"
}

func formatMoneyCents(val interface{}) (string, float64, bool) {
	if val == nil {
		return "", 0, false
	}
	var c int64
	switch v := val.(type) {
	case int64:
		c = v
	case int:
		c = int64(v)
	case float64:
		c = int64(v)
	case string:
		c, _ = strconv.ParseInt(v, 10, 64)
	default:
		return "", 0, false
	}
	sign := ""
	absCents := c
	if absCents < 0 {
		sign = "-"
		absCents = -absCents
	}
	rubles := absCents / 100
	kopecks := absCents % 100
	strVal := fmt.Sprintf("%s%d.%02d", sign, rubles, kopecks)
	floatVal := float64(c) / 100.0
	return strVal, floatVal, true
}

func formatConversionVal(val interface{}) (string, float64, bool) {
	if val == nil {
		return "", 0, false
	}
	switch v := val.(type) {
	case float64:
		return fmt.Sprintf("%.2f", v), v, true
	case int64:
		return fmt.Sprintf("%.2f", float64(v)), float64(v), true
	case int:
		return fmt.Sprintf("%.2f", float64(v)), float64(v), true
	default:
		return "", 0, false
	}
}

func formatCountVal(val interface{}) (string, float64, bool) {
	if val == nil {
		return "", 0, false
	}
	switch v := val.(type) {
	case int64:
		return strconv.FormatInt(v, 10), float64(v), true
	case int:
		return strconv.Itoa(v), float64(v), true
	case float64:
		return strconv.FormatInt(int64(v), 10), v, true
	case string:
		return v, 0, true
	default:
		return "", 0, false
	}
}

func neutralizeFormulaString(s string) string {
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "=") || strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "@") {
		return "'" + s
	}
	return s
}
