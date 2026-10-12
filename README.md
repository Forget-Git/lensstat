# lensstat — 照片 EXIF 统计工具

一个用 Go 编写的桌面客户端（GUI），递归扫描相册目录下的所有 JPG/JPEG 照片，读取 EXIF 信息（设备、焦距、等效焦距、拍摄时间、镜头型号），导出为 Excel，并自动统计使用最多的焦段。

## 功能特性

- **递归扫描**：自动遍历相册根目录下所有层级的子文件夹。
- **扩展名兼容**：只处理 `.jpg` / `.jpeg`，大小写不敏感（`.JPG`、`.Jpeg` 等均支持）。
- **EXIF 提取**：设备品牌、设备型号、实际焦距、等效焦距、拍摄时间、镜头型号。
- **焦段统计**：优先使用等效焦距，其次实际焦距；四舍五入到整数；按使用数量降序排列。
- **原生文件对话框**：目录选择 / 保存位置使用系统原生对话框（macOS / Windows / Linux）。
- **Excel 双工作表**：
  - `照片信息`：10 列明细（表头加粗、冻结首行、自动筛选、合理列宽）。
  - `焦段统计`：焦段与数量（首行即使用最多的焦段）。
- **异常处理**：文件打开失败 / EXIF 解析失败写入「备注」列并继续；无焦距不参与统计；无拍摄时间留空。

## 技术栈

- Go 1.24.0
- `github.com/rwcarlsen/goexif/exif` — 读取 EXIF
- `github.com/xuri/excelize/v2` — 生成 Excel
- `fyne.io/fyne/v2` — 桌面 GUI 窗口与提示弹窗
- `github.com/sqweek/dialog` — 系统原生文件/目录选择器

## 构建与运行

```bash
# 启动图形界面
go run .

# 编译为可执行文件
go build -o lensstat .
./lensstat
```

### 跨平台说明

- **macOS / Linux**：直接 `go build` 即可。
- **Windows**：在 Windows 上直接 `go build -o lensstat.exe .`，双击运行。Fyne 依赖 cgo（OpenGL），建议在 Windows 本机编译；若需交叉编译需配置 mingw-w64 工具链。
- macOS 编译时可能出现 `allowedFileTypes` 弃用警告（来自 `sqweek/dialog` 库内部），不影响功能。

## 使用步骤

1. 点击「选择目录」，用系统对话框选中相册根目录（会递归扫描所有子文件夹）。
2. 点击「保存位置」，选择导出 Excel 的保存路径与文件名（默认文件名 `照片EXIF.xlsx`，可修改）。
3. 点击「开始扫描」，日志区实时显示进度，完成后弹窗提示。
4. 打开生成的 Excel 查看明细与焦段统计。

> 若未单独指定保存位置，默认输出到所选相册目录下，文件名为 `照片EXIF.xlsx`。

## 字段对照

| 输出列 | EXIF 标签 | 说明 |
| --- | --- | --- |
| 设备品牌 | `Make` (0x010F) | 字符串 |
| 设备型号 | `Model` (0x0110) | 字符串 |
| 实际焦距 | `FocalLength` (0x920A) | RATIONAL，保留 2 位小数 |
| 等效焦距 | `FocalLengthIn35mmFilm` (0xA405) | SHORT，转为整数 |
| 拍摄时间 | `DateTimeOriginal` (0x9003) | 优先 |
| 拍摄时间(次选) | `DateTimeDigitized` (0x9004) | 次选 |
| 拍摄时间(末选) | `DateTime` (0x0132) | 最后选 |
| 镜头型号 | `LensModel` (0xA434) | 字符串 |

- 拍摄时间原始格式 `2006:01:02 15:04:05`，输出格式化为 `2006-01-02 15:04:05`。
- 手机照片建议优先看「等效焦距(mm)」；实际焦距仅作回退。

## 注意事项

- 只扫描 `.jpg` 和 `.jpeg`，其他格式忽略。
- 照片数量较大时运行时间稍长，几千到几万张可正常处理。
- 位于 iCloud、OneDrive 等云盘中的照片需先下载原图到本地（云占位文件无法读取 EXIF）。
- 截图、微信 / QQ 保存图、编辑后导出的 JPG 通常无 EXIF，会在「备注」列体现。

## 项目结构

```
lensstat/
├── main.go         # 主程序（GUI + EXIF 解析 + Excel 导出）
├── main_test.go    # 核心逻辑单元测试
├── go.mod          # 模块定义（module lensstat, go 1.24.0）
├── go.sum
└── README.md
```
