#!/usr/bin/env python3
"""
修复TMD下载的txt文件中的HTML实体编码问题
将 &amp; 等HTML实体解码为正常字符，同时保持文件修改日期不变
支持多线程并行处理
"""

import os
import sys
import html
import argparse
from pathlib import Path
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor, as_completed
from threading import Lock


print_lock = Lock()


def safe_print(message: str):
    """线程安全的打印函数"""
    with print_lock:
        print(message)


def fix_html_entities_in_file(file_path: Path, dry_run: bool = False) -> tuple[bool, bool]:
    """
    修复单个文件中的HTML实体编码
    
    Args:
        file_path: 文件路径
        dry_run: 是否为试运行模式（不实际修改文件）
    
    Returns:
        (是否成功处理, 是否进行了修复)
    """
    try:
        # 读取文件内容
        content = file_path.read_text(encoding='utf-8')
        
        # 解码HTML实体
        decoded_content = html.unescape(content)
        
        # 检查是否有变化
        if content == decoded_content:
            return True, False
        
        if dry_run:
            safe_print(f"[试运行] 需要修复: {file_path}")
            return True, True
        
        # 保存原始修改时间
        original_stat = file_path.stat()
        original_mtime = original_stat.st_mtime
        
        # 写入修复后的内容
        file_path.write_text(decoded_content, encoding='utf-8')
        
        # 恢复原始修改时间
        os.utime(file_path, (original_stat.st_atime, original_mtime))
        
        safe_print(f"已修复: {file_path}")
        return True, True
        
    except Exception as e:
        safe_print(f"处理失败 {file_path}: {e}")
        return False, False


def scan_and_fix_directory(directory: Path, dry_run: bool = False, workers: int = 4) -> tuple[int, int, int]:
    """
    扫描目录并修复所有txt文件（多线程版本）
    
    Args:
        directory: 要扫描的目录
        dry_run: 是否为试运行模式
        workers: 线程池大小
    
    Returns:
        (处理的文件数, 修复的文件数, 失败数)
    """
    # 收集所有txt文件
    txt_files = list(directory.rglob("*.txt"))
    total = len(txt_files)
    
    if total == 0:
        return 0, 0, 0
    
    fixed = 0
    failed = 0
    
    # 使用线程池并行处理
    with ThreadPoolExecutor(max_workers=workers) as executor:
        # 提交所有任务
        future_to_file = {
            executor.submit(fix_html_entities_in_file, f, dry_run): f 
            for f in txt_files
        }
        
        # 处理结果
        for future in as_completed(future_to_file):
            success, was_fixed = future.result()
            if not success:
                failed += 1
            elif was_fixed:
                fixed += 1
    
    return total, fixed, failed


def main():
    parser = argparse.ArgumentParser(
        description="修复TMD下载的txt文件中的HTML实体编码问题（如 &amp; -> &）",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  python fix_html_entities.py "下载目录"
  python fix_html_entities.py "下载目录" -n
  python fix_html_entities.py "下载目录" -w 8
  python fix_html_entities.py "path/to/specific.txt"
        """
    )
    parser.add_argument(
        "path",
        nargs="?",
        default=".",
        help="要扫描的目录或文件路径（默认为当前目录）"
    )
    parser.add_argument(
        "--dry-run", "-n",
        action="store_true",
        help="试运行模式，只显示哪些文件需要修复，不实际修改"
    )
    parser.add_argument(
        "--workers", "-w",
        type=int,
        default=4,
        help="线程池大小（默认为4）"
    )
    
    args = parser.parse_args()
    
    # 验证 workers 参数
    if args.workers < 1:
        print(f"错误: 线程数必须大于0，当前: {args.workers}")
        sys.exit(1)
    
    target_path = Path(args.path).resolve()
    
    if not target_path.exists():
        print(f"错误: 路径不存在: {target_path}")
        sys.exit(1)
    
    print(f"{'[试运行模式] ' if args.dry_run else ''}开始扫描: {target_path}")
    print(f"线程数: {args.workers}")
    print("-" * 50)
    
    start_time = datetime.now()
    
    if target_path.is_file():
        # 单个文件
        if target_path.suffix.lower() != '.txt':
            print(f"错误: 不是txt文件: {target_path}")
            sys.exit(1)
        processed = 1
        success, was_fixed = fix_html_entities_in_file(target_path, args.dry_run)
        failed = 0 if success else 1
        fixed = 1 if was_fixed else 0
    else:
        # 目录
        processed, fixed, failed = scan_and_fix_directory(target_path, args.dry_run, args.workers)
    
    elapsed = datetime.now() - start_time
    
    print("-" * 50)
    print(f"扫描完成: 共处理 {processed} 个文件，修复 {fixed} 个文件", end="")
    if failed > 0:
        print(f"，失败 {failed} 个", end="")
    print(f"，耗时 {elapsed.total_seconds():.2f} 秒")
    
    if args.dry_run and fixed > 0:
        print(f"\n提示: 使用以下命令实际修复文件:")
        print(f'  python "{sys.argv[0]}" "{args.path}"')


if __name__ == "__main__":
    main()
