import { cva, type VariantProps } from 'class-variance-authority'

export { default as Badge } from './Badge.vue'

/**
 * 基础形态采用资源池导航菜单（demo 同款）的计数徽章度量：
 * 等宽字体小胶囊，px-2 py-0.5 text-[11px]。
 */
export const badgeVariants = cva(
  'inline-flex items-center justify-center whitespace-nowrap shrink-0 rounded-full px-2 py-0.5 text-[11px] font-mono font-medium transition-colors',
  {
    variants: {
      variant: {
        default: 'border border-transparent bg-primary text-primary-foreground',
        secondary: 'border border-transparent bg-secondary text-secondary-foreground',
        destructive: 'border border-transparent bg-destructive text-destructive-foreground',
        outline: 'border text-foreground',
        // —— 资源池导航菜单（demo 同款配色）——
        count:
          'border-transparent bg-slate-100 dark:bg-dark-800 text-slate-600 dark:text-slate-400 group-hover:bg-primary-500/10 group-hover:text-primary-600 dark:group-hover:text-primary-400',
        'count-muted': 'border-transparent bg-slate-100 dark:bg-dark-800 text-slate-400',
        'count-active': 'border-transparent bg-primary-500/20 text-primary-700 dark:text-primary-300 font-semibold',
        'count-amber': 'border-transparent bg-amber-500/15 text-amber-700 dark:text-amber-300 font-semibold',
        'pool-count': 'border-transparent bg-slate-100 dark:bg-dark-800 text-slate-500 dark:text-slate-400',
        multiplier: 'border-transparent bg-purple-500/10 text-purple-600 dark:text-purple-400 font-bold text-[9px] px-1.5',
        rose: 'border-transparent bg-rose-500/15 text-rose-700 dark:text-rose-300',
        'platform-count': 'border-transparent bg-slate-100 dark:bg-dark-800 text-slate-400 text-[10px] px-1.5'
      }
    },
    defaultVariants: {
      variant: 'default'
    }
  }
)

export type BadgeVariants = VariantProps<typeof badgeVariants>
