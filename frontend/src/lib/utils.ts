import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * shadcn-vue 标准的类名合并工具：
 * 条件拼接 + 解决 Tailwind 工具类冲突（后者优先）。
 */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}
