import type { ButtonHTMLAttributes } from 'react';
import { Slot } from '@radix-ui/react-slot';

import { cn } from '@/lib/utils';

export type ButtonVariant = 'primary' | 'default' | 'secondary' | 'ghost' | 'destructive';
export type ButtonSize = 'sm' | 'md' | 'lg';

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  asChild?: boolean;
  variant?: ButtonVariant;
  size?: ButtonSize;
};

const variantStyles: Record<ButtonVariant, string> = {
  primary: 'bg-accent/80 text-accent-foreground hover:bg-accent/90 shadow-[0_0_25px_rgba(106,247,217,0.35)]',
  default: 'bg-accent/80 text-accent-foreground hover:bg-accent/90 shadow-[0_0_25px_rgba(106,247,217,0.35)]',
  secondary: 'border border-white/20 text-foreground hover:border-white/40 hover:bg-white/5',
  ghost: 'bg-transparent text-foreground hover:bg-white/5',
  destructive: 'bg-destructive text-destructive-foreground hover:bg-destructive/90',
};

const sizeStyles: Record<ButtonSize, string> = {
  sm: 'px-3 py-1.5 text-xs',
  md: 'px-4 py-2 text-sm',
  lg: 'px-5 py-2.5 text-sm sm:text-base',
};

export function Button({ asChild, className, variant = 'primary', size = 'md', ...props }: ButtonProps) {
  const Comp = asChild ? Slot : 'button';
  return (
    <Comp
      className={cn(
        'inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-xl font-medium transition-all focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:pointer-events-none disabled:opacity-50 ring-offset-deep-space',
        variantStyles[variant],
        sizeStyles[size],
        className,
      )}
      {...props}
    />
  );
}
