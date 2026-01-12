package main

import (
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type DependencyAnalyzer struct {
	visited     map[string]bool
	stdlib      map[string]bool
	thirdParty  map[string]bool
	internal    map[string]bool
	projectPath string
	goPath      string
	goModPath   string
}

func NewDependencyAnalyzer(projectPath string) *DependencyAnalyzer {
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		goPath = filepath.Join(os.Getenv("HOME"), "go")
	}

	// 读取 go.mod 获取模块路径
	goModPath := ""
	modData, err := os.ReadFile(filepath.Join(projectPath, "go.mod"))
	if err == nil {
		lines := strings.Split(string(modData), "\n")
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "module ") {
				goModPath = strings.TrimSpace(strings.TrimPrefix(line, "module "))
				break
			}
		}
	}

	return &DependencyAnalyzer{
		visited:     make(map[string]bool),
		stdlib:      make(map[string]bool),
		thirdParty:  make(map[string]bool),
		internal:    make(map[string]bool),
		projectPath: projectPath,
		goPath:      goPath,
		goModPath:   goModPath,
	}
}

// 判断是否是标准库
func (da *DependencyAnalyzer) isStdLib(pkg string) bool {
	// 标准库特征：不包含点号或者是以 golang.org/x/ 开头
	if !strings.Contains(strings.Split(pkg, "/")[0], ".") {
		return true
	}
	return false
}

// 判断是否是内部包
func (da *DependencyAnalyzer) isInternalPkg(pkg string) bool {
	if da.goModPath != "" {
		return strings.HasPrefix(pkg, da.goModPath)
	}
	return strings.HasPrefix(pkg, "xiaoiron.com/admin")
}

// 解析文件获取导入的包
func (da *DependencyAnalyzer) parseFile(filePath string) ([]string, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filePath, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}

	var imports []string
	for _, imp := range node.Imports {
		// 去除引号
		path := strings.Trim(imp.Path.Value, `"`)
		imports = append(imports, path)
	}

	return imports, nil
}

// 分类包
func (da *DependencyAnalyzer) classifyPackage(pkg string) {
	if da.visited[pkg] {
		return
	}
	da.visited[pkg] = true

	if da.isStdLib(pkg) {
		da.stdlib[pkg] = true
	} else if da.isInternalPkg(pkg) {
		da.internal[pkg] = true
	} else {
		da.thirdParty[pkg] = true
	}
}

// 递归分析依赖
func (da *DependencyAnalyzer) analyzeDependencies(startFile string, deep bool) error {
	imports, err := da.parseFile(startFile)
	if err != nil {
		return fmt.Errorf("解析文件 %s 失败: %v", startFile, err)
	}


	for _, pkg := range imports {
		da.classifyPackage(pkg)

		// 如果是深度分析且是内部包，继续递归
		if deep && da.isInternalPkg(pkg) {
			pkgPath := strings.TrimPrefix(pkg, da.goModPath+"/")
			fullPath := filepath.Join(da.projectPath, pkgPath)

			// 检查是否是目录
			if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
				// 查找目录中的所有 .go 文件
				files, err := filepath.Glob(filepath.Join(fullPath, "*.go"))
				if err == nil {
					for _, file := range files {
						// 跳过测试文件
						if strings.HasSuffix(file, "_test.go") {
							continue
						}
						if !da.visited[file] {
							da.visited[file] = true
							da.analyzeDependencies(file, deep)
						}
					}
				}
			}
		}
	}

	return nil
}

// 打印结果
func (da *DependencyAnalyzer) printResults(verbose bool, filterType string) {
	fmt.Println("\n==================== 依赖分析结果 ====================\n")

	// 标准库
	if len(da.stdlib) > 0 && (filterType == "all" || filterType == "stdlib") {
		fmt.Printf("📦 标准库 (%d):\n", len(da.stdlib))
		stdlib := make([]string, 0, len(da.stdlib))
		for pkg := range da.stdlib {
			stdlib = append(stdlib, pkg)
		}
		sort.Strings(stdlib)
		for _, pkg := range stdlib {
			if verbose {
				fmt.Printf("  ✓ %s\n", pkg)
			} else {
				fmt.Printf("  %s\n", pkg)
			}
		}
		fmt.Println()
	}

	// 第三方库
	if len(da.thirdParty) > 0 && (filterType == "all" || filterType == "third-party") {
		fmt.Printf("🌐 第三方库 (%d):\n", len(da.thirdParty))
		thirdParty := make([]string, 0, len(da.thirdParty))
		for pkg := range da.thirdParty {
			thirdParty = append(thirdParty, pkg)
		}
		sort.Strings(thirdParty)
		for _, pkg := range thirdParty {
			if verbose {
				fmt.Printf("  ✓ %s\n", pkg)
			} else {
				fmt.Printf("  %s\n", pkg)
			}
		}
		fmt.Println()
	}

	// 内部包
	if len(da.internal) > 0 && (filterType == "all" || filterType == "internal") {
		fmt.Printf("🏠 内部包 (%d):\n", len(da.internal))
		internal := make([]string, 0, len(da.internal))
		for pkg := range da.internal {
			internal = append(internal, pkg)
		}
		sort.Strings(internal)
		for _, pkg := range internal {
			if verbose {
				fmt.Printf("  ✓ %s\n", pkg)
			} else {
				fmt.Printf("  %s\n", pkg)
			}
		}
		fmt.Println()
	}

	// 统计
	if filterType == "all" {
		total := len(da.stdlib) + len(da.thirdParty) + len(da.internal)
		fmt.Println("==================== 统计信息 ====================")
		fmt.Printf("总计: %d 个包\n", total)
		if total > 0 {
			fmt.Printf("  - 标准库: %d (%.1f%%)\n", len(da.stdlib), float64(len(da.stdlib))/float64(total)*100)
			fmt.Printf("  - 第三方库: %d (%.1f%%)\n", len(da.thirdParty), float64(len(da.thirdParty))/float64(total)*100)
			fmt.Printf("  - 内部包: %d (%.1f%%)\n", len(da.internal), float64(len(da.internal))/float64(total)*100)
		}
		fmt.Println("===================================================")
	} else {
		// 只显示指定类型的统计
		fmt.Println("==================== 统计信息 ====================")
		switch filterType {
		case "stdlib":
			fmt.Printf("标准库: %d 个包\n", len(da.stdlib))
		case "third-party":
			fmt.Printf("第三方库: %d 个包\n", len(da.thirdParty))
		case "internal":
			fmt.Printf("内部包: %d 个包\n", len(da.internal))
		}
		fmt.Println("===================================================")
	}
}

func main() {
	// 命令行参数
	filePath := flag.String("f", "", "入口文件路径 (必填)")
	deep := flag.Bool("d", false, "深度分析，递归分析内部包的依赖")
	verbose := flag.Bool("v", false, "详细输出")
	filterType := flag.String("type", "all", "只显示指定类型的依赖: stdlib (标准库) | third-party (第三方库) | internal (内部包) | all (全部)")
	flag.Parse()

	if *filePath == "" {
		fmt.Println("错误: 请指定入口文件路径")
		fmt.Println("\n使用方法:")
		fmt.Println("  go run check_deps.go -f <入口文件路径> [-d] [-v] [-type <类型>]")
		fmt.Println("\n参数说明:")
		fmt.Println("  -f     入口文件路径 (必填)")
		fmt.Println("  -d     深度分析，递归分析内部包的依赖")
		fmt.Println("  -v     详细输出")
		fmt.Println("  -type  只显示指定类型的依赖")
		fmt.Println("         类型: stdlib (标准库) | third-party (第三方库) | internal (内部包) | all (全部，默认)")
		fmt.Println("\n示例:")
		fmt.Println("  go run check_deps.go -f service/manager/rpc/manager.go")
		fmt.Println("  go run check_deps.go -f service/manager/rpc/manager.go -d")
		fmt.Println("  go run check_deps.go -f service/admin/api/admin.go -d -v")
		fmt.Println("  go run check_deps.go -f service/manager/rpc/manager.go -type stdlib")
		fmt.Println("  go run check_deps.go -f service/manager/rpc/manager.go -type third-party")
		os.Exit(1)
	}

	// 验证 filterType
	validTypes := map[string]bool{
		"all":         true,
		"stdlib":      true,
		"third-party": true,
		"internal":    true,
	}
	if !validTypes[*filterType] {
		fmt.Printf("错误: 无效的类型 '%s'\n", *filterType)
		fmt.Println("支持的类型: stdlib, third-party, internal, all")
		os.Exit(1)
	}

	// 获取绝对路径
	absPath, err := filepath.Abs(*filePath)
	if err != nil {
		fmt.Printf("错误: 无法获取文件绝对路径: %v\n", err)
		os.Exit(1)
	}

	// 检查文件是否存在
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		fmt.Printf("错误: 文件不存在: %s\n", absPath)
		os.Exit(1)
	}

	// 获取项目根目录（假设脚本在 scripts 目录下）
	projectPath, err := os.Getwd()
	if err != nil {
		fmt.Printf("错误: 无法获取当前目录: %v\n", err)
		os.Exit(1)
	}

	// 如果当前目录是 scripts，则向上一级
	if filepath.Base(projectPath) == "scripts" {
		projectPath = filepath.Dir(projectPath)
	}

	fmt.Printf("分析文件: %s\n", absPath)
	if *deep {
		fmt.Println("模式: 深度分析（递归内部包）")
	} else {
		fmt.Println("模式: 浅层分析（仅直接依赖）")
	}

	// 创建分析器
	analyzer := NewDependencyAnalyzer(projectPath)

	// 分析依赖
	if err := analyzer.analyzeDependencies(absPath, *deep); err != nil {
		fmt.Printf("错误: %v\n", err)
		os.Exit(1)
	}

	// 打印结果
	analyzer.printResults(*verbose, *filterType)
}
