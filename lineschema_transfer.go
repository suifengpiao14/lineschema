package lineschema

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/cast"
	"github.com/suifengpiao14/funcs"
)

type TransferUnit struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

type Transfer struct {
	Src TransferUnit `json:"src"`
	Dst TransferUnit `json:"dst"`
}

// 外界不可以直接初始化,
type Transfers []Transfer

func NewTransfer() (transfer Transfers) {
	return Transfers{}
}

// 新增，存在替换
func (transfer *Transfers) Replace(transferItems ...Transfer) {
	for _, transferItem := range transferItems {
		exists := false
		for i, item := range *transfer {
			if item.Dst.Path == transferItem.Dst.Path {
				(*transfer)[i] = transferItem
				exists = true
				break
			}
		}
		if !exists {
			*transfer = append(*transfer, transferItem)
		}
	}
}

func (transfer Transfers) Reverse() (reversedTransfer Transfers) {
	reversedTransfer = Transfers{}
	for _, item := range transfer {
		refersedItem := Transfer{
			Src: item.Dst,
			Dst: item.Src,
		}
		reversedTransfer = append(reversedTransfer, refersedItem)
	}
	return reversedTransfer
}

// addTransferModify 在来源路径上增加上目标类型转换函数
func (t Transfers) addTransferModify() (newT Transfers) {
	newT = make(Transfers, 0)
	for _, transfer := range t {
		transferFunc, ok := DefaultTransferFuncs.GetByType(transfer.Dst.Type)
		if ok {
			transfer.Src.Path = fmt.Sprintf("%s%s", transfer.Src.Path, transferFunc.ConvertFn) //存在映射函数,则修改,否则保持原样
		}
		newT = append(newT, transfer)
	}

	return newT

}

type transfersModel struct {
	keys transfersKeys
	m    map[string]any
}

type transfersKeys []string

func (tks *transfersKeys) AppendIgnore(key string) { // 存在忽略
	for _, existsKey := range *tks {
		if existsKey == key {
			return
		}

	}
	*tks = append(*tks, key)
}

func (t Transfers) String() (gjsonPath string) {
	newT := t.addTransferModify()
	m := &transfersModel{
		keys: make([]string, 0),
		m:    make(map[string]any),
	}
	if len(newT) == 0 {
		return ""
	}
	if len(newT) == 1 && newT[0].Dst.Path == "" { // 后续代码默认为对象，在开头增加 . 如只有一个，则不可默认，源字符串输出即可
		return newT[0].Src.Path
	}
	for _, item := range newT {
		dst := item.Dst
		dstPath := strings.ReplaceAll(dst.Path, "@this.", "") // 目标地址 @this. 删除
		dstPath = strings.TrimPrefix(dstPath, ".")
		if !strings.HasPrefix(dstPath, "#") {
			dstPath = fmt.Sprintf(".%s", dstPath) // 非数组，统一标准化前缀
		}

		arr := strings.Split(dstPath, ".")
		l := len(arr)
		ref := m
		for i, key := range arr {
			if l == i+1 { // 处理最后一个
				if (*ref).m[key] == nil {
					(*ref).keys.AppendIgnore(key)
					(*ref).m[key] = item.Src.Path // 第一次默认设置为字符串类型, 如果已经存在,不再修改成字符串(//当类型为 object,array 的在后面,之前有子元素时,忽略)
				}

				continue
			}
			var ok bool
			if _, ok = (*ref).m[key]; !ok {
				(*ref).keys.AppendIgnore(key)
				(*ref).m[key] = &transfersModel{
					keys: make([]string, 0),
					m:    make(map[string]any),
				}
			}
			if ok {
				_, ok = (*ref).m[key].(*transfersModel) //检验类型( //当类型为 object,array 的在前面先设置时 (fullname=items, type=array )其类型不为map)
			}
			if !ok {
				(*ref).keys.AppendIgnore(key)
				(*ref).m[key] = &transfersModel{
					keys: make([]string, 0),
					m:    make(map[string]any),
				}
			}
			ref = (*ref).m[key].(*transfersModel) // 本次递进一定成功
		}

	}
	w, _ := t.recursionWrite(m, false, 0)
	gjsonPath = w.String()

	return gjsonPath
}

// 生成路径
func (t Transfers) recursionWrite(m *transfersModel, parentIsArray bool, depth int) (w bytes.Buffer, childrenIsArray bool) {
	writeComma := false
	for _, k := range m.keys {
		v := (*m).m[k]
		if writeComma {
			w.WriteString(",")
		}
		writeComma = true
		ref, ok := v.(*transfersModel)
		if !ok {
			switch k {
			case "#":
				childrenIsArray = true
				w.WriteString(cast.ToString(v))
			case "":
				w.WriteString(cast.ToString(v))

			default:
				w.WriteString(fmt.Sprintf("%s:%s", k, cast.ToString(v)))
			}
			continue
		}
		var subw bytes.Buffer
		currentIsArray := k == "#"
		if currentIsArray {
			depth++
		}
		subw, subChildrenIsArray := t.recursionWrite(ref, currentIsArray, depth) //isWrapBraces 必须使用外出定义,才能返回true到上一个函数
		subwKey := subw.String()
		if !subChildrenIsArray { //不会被{}包裹,则使用{} 将子内容包裹，表示对象整体(@group 执行后会自动生成{},此处要排除这种情况)
			subwKey = fmt.Sprintf("{%s}", subwKey)
			if parentIsArray {
				subwKey = fmt.Sprintf("%s|@groupPlus:%d", subwKey, depth-1) // 上一级也为数组时，需要包裹到[]中
			}
		}
		var subStr string
		switch k {
		case "#":
			childrenIsArray = true
			subStr = fmt.Sprintf("%s|@groupPlus:%d", subwKey, depth-1)
		case "":
			subStr = subwKey
		default:
			subStr = fmt.Sprintf("%s:%s", k, subwKey)
		}
		w.WriteString(subStr)
	}
	return w, childrenIsArray
}

// PathModifyFn 路径修改函数
type PathModifyFn func(path string) (newPath string)

// PathModifyFnCameCase 将路径改成小驼峰格式
func PathModifyFnCameCase(path string) (newPath string) {
	newPath = funcs.CamelCase(path, false, false)
	return
}

// PathModifyFnSnakeCase 将路径转为下划线格式
func PathModifyFnSnakeCase(path string) (newPath string) {
	arr := strings.Split(path, ".")
	l := len(arr)
	newArr := make([]string, l)
	for i := 0; i < l; i++ {
		newArr[i] = funcs.SnakeCase(arr[i])
	}
	newPath = strings.Join(newArr, ".")
	return
}

// PathModifyFnLower 将路径转为小写格式
func PathModifyFnLower(path string) (newPath string) {
	return strings.ToLower(path)
}

// PathModifyFnTrimPrefixFn 生成剔除前缀修改函数
func PathModifyFnTrimPrefixFn(prefix string) (pathModifyFn PathModifyFn) {
	return func(path string) (newPath string) {
		return strings.TrimPrefix(path, prefix)
	}
}

// ModifyPath 修改转换路径
func (t Transfers) ModifyDstPath(dstPathModifyFns ...PathModifyFn) (nt Transfers) {
	nt = make(Transfers, 0)
	for _, l := range t {
		src := l.Src
		dst := l.Dst
		for _, fn := range dstPathModifyFns {
			if fn != nil {
				dst.Path = fn(dst.Path)
			}

		}
		item := Transfer{
			Src: src,
			Dst: dst,
		}
		nt.Replace(item)
	}
	return nt
}
func (t Transfers) ModifySrcPath(srcPathModifyFns ...PathModifyFn) (nt Transfers) {
	nt = make(Transfers, 0)
	for _, l := range t {
		src := l.Src
		dst := l.Dst
		for _, fn := range srcPathModifyFns {
			if fn != nil {
				src.Path = fn(src.Path)
			}
		}
		item := Transfer{
			Src: src,
			Dst: dst,
		}
		nt.Replace(item)
	}
	return nt
}

type TransferFunc struct {
	Type      string `json:"type"`      // 对应类型
	ConvertFn string `json:"convertFn"` // 转换函数名称
}
type TransferFuncs []TransferFunc

func (ts TransferFuncs) GetByType(typ string) (t *TransferFunc, ok bool) {
	for _, transfer := range ts {
		if strings.EqualFold(transfer.Type, typ) {
			return &transfer, true
		}
	}
	return nil, false
}

// DefaultTransferFuncs schema format 转类型
var DefaultTransferFuncs = TransferFuncs{
	{Type: "int", ConvertFn: ".@tonum"},
	{Type: "integer", ConvertFn: ".@tonum"},
	{Type: "number", ConvertFn: ".@tonum"},
	{Type: "float", ConvertFn: ".@tonum"},
	{Type: "bool", ConvertFn: ".@tobool"},
	{Type: "boolean", ConvertFn: ".@tobool"},
	{Type: "string", ConvertFn: ".@tostring"},
}

// ToGoTypeTransfer 根据go结构体json tag以及类型生成转换
func ToGoTypeTransfer(dst any) (lineschemaTransfer Transfers) {
	if dst == nil {
		return nil
	}
	rv := reflect.Indirect(reflect.ValueOf(dst))
	rt := rv.Type()
	return toGoTypeTransfer(rt, "@this")
}

func toGoTypeTransfer(rt reflect.Type, prefix string) (lineschemaTransfer Transfers) {
	switch rt.Kind() {
	case reflect.Array, reflect.Slice:
		lineschemaTransfer = toGoTypeTransfer(rt.Elem(), fmt.Sprintf("%s.#", prefix))
	case reflect.Struct:
		lineschemaTransfer = str2StructTransfer(rt, prefix)
	case reflect.Int64, reflect.Float64, reflect.Int:
		lineschemaTransfer = str2SimpleTypeTransfer("number", prefix)
	case reflect.Bool:
		lineschemaTransfer = str2SimpleTypeTransfer("bool", prefix)
	case reflect.String:
		lineschemaTransfer = str2SimpleTypeTransfer("string", prefix)
	}

	for i := range lineschemaTransfer {
		t := &lineschemaTransfer[i]
		// 删除前缀 @this
		t.Dst.Path = strings.TrimPrefix(t.Dst.Path, "@this")
	}

	return lineschemaTransfer
}

func str2SimpleTypeTransfer(typ string, path string) (lineschemaTransfer Transfers) {
	if path == "" {
		path = "@this"
	}
	return Transfers{
		Transfer{
			Dst: TransferUnit{
				Path: path,
				Type: typ,
			},
			Src: TransferUnit{
				Path: path,
				Type: "string",
			},
		},
	}
}

func str2StructTransfer(rt reflect.Type, prefix string) (transfers Transfers) {
	if rt.Kind() != reflect.Struct {
		return nil
	}
	if prefix != "" {
		prefix = strings.TrimRight(prefix, ".")
		prefix = fmt.Sprintf("%s.", prefix)
	}
	transfers = make(Transfers, 0)
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		typ := field.Type.String()
		tag := field.Tag.Get("json")
		if tag == "-" {
			continue // Skip fields without json tag or with "-" tag
		}

		isString := strings.Contains(tag, ",string")
		if isString {
			typ = "string"
		}
		commIndex := strings.Index(tag, ",")
		if commIndex > -1 {
			tag = tag[:commIndex] // 取,前的内容
		}

		fieldType := field.Type
		filedTK := field.Type.Kind()
		switch filedTK {
		case reflect.Slice, reflect.Array, reflect.Struct:
			subPrefix := fmt.Sprintf("%s%s", prefix, tag)
			subTransfer := str2StructTransfer(fieldType, subPrefix)
			transfers.Replace(subTransfer...)
			continue // 复合类型，只收集子值
		}
		if tag == "" {
			tag = field.Name // 根据json.Umarsh/Marsh 发现未写json tag时，默认使用列名称，此处兼容保持一致
		}
		path := fmt.Sprintf("%s%s", prefix, tag)
		linschemaT := Transfer{
			Dst: TransferUnit{
				Path: path,
				Type: typ,
			},
			Src: TransferUnit{
				Path: path,
				Type: "string",
			},
		}
		transfers = append(transfers, linschemaT)
	}

	return transfers
}
