import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const badgeVariants = cva(
  'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2',
  {
    variants: {
      variant: {
        default:
          'border-transparent bg-spotify-green text-black hover:bg-spotify-green/80',
        secondary:
          'border-transparent bg-gray-700 text-white hover:bg-gray-600',
        destructive:
          'border-transparent bg-red-500 text-white hover:bg-red-500/80',
        outline: 'text-gray-300 border-gray-600',
        success:
          'border-transparent bg-green-600 text-white hover:bg-green-500/80',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  }
)

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return (
    <div className={cn(badgeVariants({ variant }), className)} {...props} />
  )
}

export { Badge, badgeVariants }
