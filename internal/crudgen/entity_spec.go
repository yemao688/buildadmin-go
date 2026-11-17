package crudgen

import (
	crudmodel "buildadmin-go/internal/pkg/crudmodel"
	"fmt"
	"strings"
)

// buildSpecEntityStruct 按 spec 生成实体结构体源码。
//
// 它取代 gorm.io/gen 的数据库反查：spec 已经是表结构的事实源，实体 tag 直接由
// spec 推导，在线生成与离线渲染才会逐字节一致；也避免库结构与 spec 漂移时实体
// 跟着库走。
func buildSpecEntityStruct(table crudmodel.Table, fields []crudmodel.Field, className string, typeOverrides map[string]string) (string, error) {
	if className == "" {
		return "", fmt.Errorf("entity struct name is required")
	}
	indexTags := buildEntityIndexTags(table.Indexes)
	var builder strings.Builder
	builder.WriteString("// " + className)
	if comment := getComment(table.Comment); comment != "" {
		builder.WriteString(" " + comment)
	}
	builder.WriteString("\n")
	builder.WriteString("type " + className + " struct {\n")
	for _, field := range fields {
		goType := typeOverrides[field.Name]
		if goType == "" {
			goType = relationColumnGoTypeFromField(field)
		}
		writeEntityField(&builder, field, goType, indexTags[field.Name])
	}
	builder.WriteString("}\n")
	return builder.String(), nil
}

// entityGoFieldName 在通用列名转换之上补齐常见缩写后缀，保证 admin_id/
// last_login_ip/json_data 这类列输出 ID/IP/JSON 形态。
func entityGoFieldName(column string) string {
	name := generatedGoFieldName(column)
	for suffix, replacement := range map[string]string{
		"Ip": "IP", "Url": "URL", "Uri": "URI", "Uuid": "UUID",
		"Api": "API", "Http": "HTTP", "Json": "JSON", "Sql": "SQL",
	} {
		if strings.HasSuffix(name, suffix) {
			name = strings.TrimSuffix(name, suffix) + replacement
		}
	}
	return name
}

// writeEntityField 输出一行实体字段：多行注释用块注释放在字段上方，单行注释
// 放行尾；tag 顺序固定，保证跨平台/多次生成逐字节一致。
func writeEntityField(builder *strings.Builder, field crudmodel.Field, goType string, indexTags []string) {
	goName := entityGoFieldName(field.Name)
	comment := sanitizeTagComment(field.Comment)
	if strings.Contains(comment, "\n") {
		builder.WriteString("\t/*\n")
		for _, line := range strings.Split(comment, "\n") {
			builder.WriteString("\t" + line + "\n")
		}
		builder.WriteString("\t*/\n")
	}
	builder.WriteString("\t" + goName + " " + goType + " `" + buildEntityGormTag(field, indexTags) + " json:\"" + field.Name + "\"`")
	if !strings.Contains(comment, "\n") && comment != "" {
		builder.WriteString(" // " + comment)
	}
	builder.WriteString("\n")
}

// buildEntityGormTag 组装 gorm tag；字段顺序固定为
// column/default/type/not null/primaryKey/autoIncrement/index/comment。
func buildEntityGormTag(field crudmodel.Field, indexTags []string) string {
	parts := []string{"column:" + field.Name}
	if defaultTag := specDefaultGormTag(field); defaultTag != "" {
		parts = append(parts, defaultTag)
	}
	sqlType := analyseFieldDataType(field)
	if field.Unsigned {
		sqlType += " unsigned"
	}
	parts = append(parts, "type:"+sqlType)
	if !field.Null && !field.PrimaryKey {
		parts = append(parts, "not null")
	}
	if field.PrimaryKey {
		parts = append(parts, "primaryKey")
	}
	if field.AutoIncrement {
		parts = append(parts, "autoIncrement:true")
	}
	parts = append(parts, indexTags...)
	if comment := sanitizeTagComment(field.Comment); comment != "" {
		// 多行注释已经写在字段上方，tag 里只保留单行内容，避免 tag 里出现换行。
		parts = append(parts, "comment:"+strings.ReplaceAll(comment, "\n", " "))
	}
	return "gorm:\"" + strings.Join(parts, ";") + "\""
}

// buildEntityIndexTags 把 spec 的 indexes 映射成字段上的 gorm index tag：
// map[列名] -> 按声明顺序的 tag 片段（唯一索引 uniqueIndex，普通索引 index）。
func buildEntityIndexTags(indexes []crudmodel.IndexSpec) map[string][]string {
	tags := make(map[string][]string, len(indexes))
	for _, index := range indexes {
		prefix := "index:"
		if index.Unique {
			prefix = "uniqueIndex:"
		}
		for position, column := range index.Columns {
			name := column
			if bracket := strings.IndexByte(name, '('); bracket >= 0 {
				name = name[:bracket]
			}
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			tag := fmt.Sprintf("%s%s,priority:%d", prefix, index.Name, position+1)
			tags[name] = append(tags[name], tag)
		}
	}
	return tags
}

// sanitizeTagComment 让注释可以安全放进 gorm tag：tag 以 ';' 分隔、结构体
// tag 以反引号包裹，因此这两类字符替换为全角/单引号。
func sanitizeTagComment(comment string) string {
	comment = strings.ReplaceAll(comment, "`", "'")
	return strings.ReplaceAll(comment, ";", "；")
}
