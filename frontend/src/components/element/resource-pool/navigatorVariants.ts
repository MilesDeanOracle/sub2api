import { cva, type VariantProps } from 'class-variance-authority'

/**
 * 资源池导航菜单的 feature 级变体（类名逐字取自 resource_pool_demo.html）。
 * 与 shadcn Sidebar 块的做法一致：菜单行/药丸拥有自己的 cva 变体，
 * 而不是把设计硬塞进通用 Button。
 */

/** 快捷筛选药丸：全部 / 正常 / 受限异常 */
export const filterPillVariants = cva(
  'px-2 py-0.5 rounded-md transition-colors text-[11px]',
  {
    variants: {
      tone: { primary: '', emerald: '', amber: '' },
      active: { true: '', false: '' }
    },
    compoundVariants: [
      {
        tone: 'primary',
        active: false,
        class: 'text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200'
      },
      {
        tone: 'primary',
        active: true,
        class: 'bg-primary-500/10 text-primary-600 dark:text-primary-400 font-semibold'
      },
      {
        tone: 'emerald',
        active: false,
        class: 'text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200'
      },
      {
        tone: 'emerald',
        active: true,
        class: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 font-semibold'
      },
      {
        tone: 'amber',
        active: false,
        class: 'text-slate-500 hover:text-slate-800 dark:text-slate-400 dark:hover:text-slate-200'
      },
      {
        tone: 'amber',
        active: true,
        class: 'bg-amber-500/10 text-amber-600 dark:text-amber-400 font-semibold'
      }
    ],
    defaultVariants: {
      tone: 'primary',
      active: false
    }
  }
)

export type FilterPillVariants = VariantProps<typeof filterPillVariants>

/** 全局视图行：全部渠道账号(teal) / 待分配账号(amber) / 限流与异常(rose) */
export const navItemVariants = cva(
  'w-full flex items-center justify-between px-2.5 py-2 rounded-xl text-xs border transition-all text-left group',
  {
    variants: {
      tone: { teal: '', amber: '', rose: '' },
      active: { true: '', false: '' }
    },
    compoundVariants: [
      {
        tone: 'teal',
        active: false,
        class: 'text-slate-600 dark:text-slate-300 hover:bg-slate-100/70 dark:hover:bg-dark-800/60 border-transparent'
      },
      {
        tone: 'teal',
        active: true,
        class:
          'bg-primary-500/10 dark:bg-primary-500/15 text-primary-700 dark:text-primary-300 font-medium border-primary-500/30'
      },
      {
        tone: 'amber',
        active: false,
        class: 'text-slate-600 dark:text-slate-300 hover:bg-slate-100/70 dark:hover:bg-dark-800/60 border-transparent'
      },
      {
        tone: 'amber',
        active: true,
        class:
          'bg-amber-500/10 dark:bg-amber-500/15 text-amber-800 dark:text-amber-300 font-medium border-amber-500/30'
      },
      {
        tone: 'rose',
        active: false,
        class: 'text-slate-600 dark:text-slate-300 hover:bg-slate-100/70 dark:hover:bg-dark-800/60 border-transparent'
      },
      {
        tone: 'rose',
        active: true,
        class:
          'bg-rose-500/10 dark:bg-rose-500/15 text-rose-800 dark:text-rose-300 font-medium border-rose-500/30'
      }
    ],
    defaultVariants: {
      tone: 'teal',
      active: false
    }
  }
)

export type NavItemVariants = VariantProps<typeof navItemVariants>

/** 平台资源池行 */
export const poolItemVariants = cva(
  'relative flex items-center justify-between px-2.5 py-2 rounded-xl text-xs border cursor-pointer transition-all group',
  {
    variants: {
      active: {
        true:
          'bg-primary-500/10 dark:bg-primary-500/20 text-slate-900 dark:text-white border-primary-500/40 shadow-sm',
        false:
          'text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-dark-800/60 border-transparent'
      }
    },
    defaultVariants: {
      active: false
    }
  }
)

export type PoolItemVariants = VariantProps<typeof poolItemVariants>
