package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	fyne "fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/rwcarlsen/goexif/exif"
	nativedlg "github.com/sqweek/dialog"
	"github.com/xuri/excelize/v2"
)

// ===== 配置 =====
// defaultOutputName：导出文件默认名（用户可在“保存位置”对话框中修改）。
const defaultOutputName = "照片EXIF.xlsx"

// outputXlsx：最终保存路径，由用户在界面中选择（含文件名）。
var outputXlsx = ""

// ===== 数据结构 =====
type PhotoInfo struct {
	FileName      string // 文件名
	FullPath      string // 完整路径
	Make          string // 设备品牌
	Model         string // 设备型号
	Device        string // 品牌 + 型号
	FocalLength   string // 实际焦距(mm)
	FocalLength35 string // 等效焦距(mm)
	DateTime      string // 拍摄时间
	LensModel     string // 镜头型号
	Err           string // 备注/错误信息
}

func main() {
	myApp := app.New()
	window := myApp.NewWindow("照片 EXIF 统计工具")
	window.Resize(fyne.NewSize(760, 560))
	window.CenterOnScreen()

	var photoDir string

	dirEntry := widget.NewEntry()
	dirEntry.SetPlaceHolder("点击右侧按钮选择相册目录，或手动输入路径")
	outEntry := widget.NewEntry()
	outEntry.SetPlaceHolder("点击右侧按钮选择保存位置（默认 " + defaultOutputName + "）")

	logBox := widget.NewMultiLineEntry()
	logBox.SetMinRowsVisible(10)
	logBox.Wrapping = fyne.TextWrapWord
	logBox.SetText("就绪。\n")

	appendLog := func(msg string) {
		fyne.Do(func() {
			logBox.SetText(logBox.Text + msg + "\n")
		})
	}

	// 选择相册目录（系统原生对话框）
	dialogDir := func() {
		sel, err := nativedlg.
			Directory().
			Title("选择相册目录（递归扫描所有子文件夹）").
			SetStartDir(photoDir).
			Browse()
		if err != nil {
			if !errors.Is(err, nativedlg.ErrCancelled) {
				dialog.ShowError(err, window)
			}
			return
		}
		photoDir = sel
		dirEntry.SetText(photoDir)
		// 若未单独指定输出，默认输出到相册目录下
		if outEntry.Text == "" {
			outputXlsx = filepath.Join(photoDir, defaultOutputName)
			outEntry.SetText(outputXlsx)
		}
	}

	// 选择保存位置（系统原生对话框，可指定文件名，默认 defaultOutputName）
	dialogOut := func() {
		startDir := photoDir
		if startDir == "" {
			startDir = "."
		}
		sel, err := nativedlg.
			File().
			Title("选择导出 Excel 的保存位置").
			SetStartDir(startDir).
			SetStartFile(defaultOutputName).
			Save()
		if err != nil {
			if !errors.Is(err, nativedlg.ErrCancelled) {
				dialog.ShowError(err, window)
			}
			return
		}
		outputXlsx = sel
		outEntry.SetText(outputXlsx)
	}

	startBtn := widget.NewButton("开始扫描", nil)
	startBtn.OnTapped = func() {
		if strings.TrimSpace(dirEntry.Text) == "" {
			dialog.ShowError(fmt.Errorf("请先选择相册目录"), window)
			return
		}
		photoDir = strings.TrimSpace(dirEntry.Text)
		if strings.TrimSpace(outEntry.Text) == "" {
			outputXlsx = filepath.Join(photoDir, defaultOutputName)
			outEntry.SetText(outputXlsx)
		} else {
			outputXlsx = strings.TrimSpace(outEntry.Text)
		}

		startBtn.Disable()
		go func() {
			defer fyne.Do(func() { startBtn.Enable() })
			count, err := process(photoDir, outputXlsx, appendLog)
			fyne.Do(func() {
				if err != nil {
					dialog.ShowError(err, window)
					return
				}
				dialog.ShowInformation("完成",
					fmt.Sprintf("共发现 %d 张 JPG\n已保存到：%s", count, outputXlsx), window)
			})
		}()
	}

	dirRow := container.NewBorder(nil, nil, nil,
		widget.NewButton("选择目录", dialogDir), dirEntry)
	outRow := container.NewBorder(nil, nil, nil,
		widget.NewButton("保存位置", dialogOut), outEntry)

	content := container.NewVBox(
		widget.NewLabel("相册目录（递归扫描所有子文件夹）"),
		dirRow,
		widget.NewLabel("导出 Excel 文件（可修改文件名）"),
		outRow,
		startBtn,
		widget.NewLabel("日志"),
		logBox,
	)

	window.SetContent(content)
	window.ShowAndRun()
}

// process 扫描目录、读取 EXIF、导出 Excel，并通过 onLog 回调输出进度。
func process(dir, out string, onLog func(string)) (int, error) {
	start := time.Now()
	onLog("开始扫描目录：" + dir)

	files, err := collectJpgFiles(dir)
	if err != nil {
		return 0, fmt.Errorf("扫描目录失败：%w", err)
	}
	onLog(fmt.Sprintf("共发现 %d 张 JPG 文件，开始读取 EXIF...", len(files)))

	photos := make([]PhotoInfo, 0, len(files))
	for i, path := range files {
		if i > 0 && i%500 == 0 {
			onLog(fmt.Sprintf("已处理 %d / %d ...", i, len(files)))
		}
		photos = append(photos, readPhoto(path))
	}

	outputXlsx = out
	if err := writeExcel(photos); err != nil {
		return 0, fmt.Errorf("写入 Excel 失败：%w", err)
	}

	onLog(fmt.Sprintf("完成：共发现 %d 张 JPG", len(photos)))
	onLog("已保存到：" + out)
	onLog(fmt.Sprintf("耗时 %s", time.Since(start)))
	return len(photos), nil
}

// collectJpgFiles 递归收集目录下所有 .jpg / .jpeg 文件（扩展名大小写不敏感）。
func collectJpgFiles(root string) ([]string, error) {
	var result []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过无法访问的子项，不中断整体流程
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".jpg" || ext == ".jpeg" {
			result = append(result, path)
		}
		return nil
	})
	return result, err
}

// readPhoto 读取单个照片的 EXIF 信息。
func readPhoto(path string) PhotoInfo {
	info := PhotoInfo{
		FileName: filepath.Base(path),
		FullPath: path,
	}

	f, err := os.Open(path)
	if err != nil {
		info.Err = "打开文件失败: " + err.Error()
		return info
	}
	defer f.Close()

	x, err := exif.Decode(f)
	if err != nil {
		info.Err = "无 EXIF 或解析失败: " + err.Error()
		return info
	}

	info.Make = getString(x, exif.Make)
	info.Model = getString(x, exif.Model)
	info.Device = strings.TrimSpace(info.Make + " " + info.Model)
	info.LensModel = getString(x, exif.LensModel)

	if fl := getFloat(x, exif.FocalLength); fl >= 0 {
		info.FocalLength = fmt.Sprintf("%.2f", fl)
	}
	if fl35 := getInt(x, exif.FocalLengthIn35mmFilm); fl35 >= 0 {
		info.FocalLength35 = strconv.Itoa(fl35)
	}
	info.DateTime = getDateTime(x)

	return info
}

func getString(x *exif.Exif, tag exif.FieldName) string {
	if v, err := x.Get(tag); err == nil {
		if s, err := v.StringVal(); err == nil {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func getFloat(x *exif.Exif, tag exif.FieldName) float64 {
	if v, err := x.Get(tag); err == nil {
		if r, err := v.Rat(0); err == nil && r != nil {
			if f, ok := r.Float64(); ok {
				return f
			}
		}
	}
	return -1
}

func getInt(x *exif.Exif, tag exif.FieldName) int {
	if v, err := x.Get(tag); err == nil {
		if i, err := v.Int(0); err == nil {
			return i
		}
	}
	return -1
}

const exifTimeLayout = "2006:01:02 15:04:05"

// getDateTime 优先拍摄时间，其次数字化时间，最后修改时间。
func getDateTime(x *exif.Exif) string {
	fields := []exif.FieldName{exif.DateTimeOriginal, exif.DateTimeDigitized, exif.DateTime}
	for _, fld := range fields {
		if v, err := x.Get(fld); err == nil {
			if s, err := v.StringVal(); err == nil {
				s = strings.TrimSpace(s)
				if t, err := time.Parse(exifTimeLayout, s); err == nil {
					return t.Format("2006-01-02 15:04:05")
				}
			}
		}
	}
	return ""
}

// writeExcel 生成包含两个工作表的 Excel 文件（保存到全局 outputXlsx）。
func writeExcel(photos []PhotoInfo) error {
	f := excelize.NewFile()
	const infoSheet = "照片信息"
	const statSheet = "焦段统计"

	f.SetSheetName("Sheet1", infoSheet)
	f.NewSheet(statSheet)

	infoHeaders := []string{"文件名", "完整路径", "设备品牌", "设备型号", "设备",
		"焦距(mm)", "等效焦距(mm)", "拍摄时间", "镜头型号", "备注"}
	if err := writeHeader(f, infoSheet, infoHeaders); err != nil {
		return err
	}
	for i, p := range photos {
		row := []interface{}{
			p.FileName, p.FullPath, p.Make, p.Model, p.Device,
			p.FocalLength, p.FocalLength35, p.DateTime, p.LensModel, p.Err,
		}
		cell := "A" + strconv.Itoa(i+2)
		if err := f.SetSheetRow(infoSheet, cell, &row); err != nil {
			return err
		}
	}
	f.SetColWidth(infoSheet, "A", "A", 24)
	f.SetColWidth(infoSheet, "B", "B", 50)
	f.SetColWidth(infoSheet, "C", "E", 22)
	f.SetColWidth(infoSheet, "F", "G", 14)
	f.SetColWidth(infoSheet, "H", "H", 20)
	f.SetColWidth(infoSheet, "I", "I", 24)
	f.SetColWidth(infoSheet, "J", "J", 30)
	f.SetPanes(infoSheet, &excelize.Panes{Freeze: true, YSplit: 1})
	f.AutoFilter(infoSheet, "A1:J1", []excelize.AutoFilterOptions{})

	counts := statFocalLength(photos)
	statHeaders := []string{"焦段(mm)", "数量"}
	if err := writeHeader(f, statSheet, statHeaders); err != nil {
		return err
	}
	for i, kv := range counts {
		row := []interface{}{strconv.Itoa(kv.focal), kv.count}
		cell := "A" + strconv.Itoa(i+2)
		if err := f.SetSheetRow(statSheet, cell, &row); err != nil {
			return err
		}
	}
	f.SetColWidth(statSheet, "A", "A", 16)
	f.SetColWidth(statSheet, "B", "B", 16)
	f.SetPanes(statSheet, &excelize.Panes{Freeze: true, YSplit: 1})
	f.AutoFilter(statSheet, "A1:B1", []excelize.AutoFilterOptions{})

	if idx, err := f.GetSheetIndex(infoSheet); err == nil {
		f.SetActiveSheet(idx)
	}
	return f.SaveAs(outputXlsx)
}

func writeHeader(f *excelize.File, sheet string, headers []string) error {
	bold, err := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	if err != nil {
		return err
	}
	for i, h := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		cell := col + "1"
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return err
		}
		if err := f.SetCellStyle(sheet, cell, cell, bold); err != nil {
			return err
		}
	}
	return nil
}

type focalCount struct {
	focal int
	count int
}

// statFocalLength 统计焦段：优先等效焦距，其次实际焦距；四舍五入取整；按数量降序。
func statFocalLength(photos []PhotoInfo) []focalCount {
	m := make(map[int]int)
	for _, p := range photos {
		focal := focalForStat(p)
		if focal < 0 {
			continue
		}
		m[focal]++
	}
	result := make([]focalCount, 0, len(m))
	for k, v := range m {
		result = append(result, focalCount{focal: k, count: v})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].count != result[j].count {
			return result[i].count > result[j].count
		}
		return result[i].focal < result[j].focal
	})
	return result
}

func focalForStat(p PhotoInfo) int {
	if p.FocalLength35 != "" {
		if v, err := strconv.Atoi(strings.TrimSpace(p.FocalLength35)); err == nil {
			return v
		}
	}
	if p.FocalLength != "" {
		if v, err := strconv.ParseFloat(strings.TrimSpace(p.FocalLength), 64); err == nil {
			return int(math.Round(v))
		}
	}
	return -1
}
