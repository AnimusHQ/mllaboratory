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
  primary: 'bg-primary text-primary-foreground shadow-glow-sm hover:bg-primary/90',
  default: 'bg-primary text-primary-foreground shadow-glow-sm hover:bg-primary/90',
  secondary: 'bg-secondary text-secondary-foreground hover:bg-secondary/90',
  ghost: 'bg-transparent text-foreground hover:bg-muted/40',
  destructive: 'bg-destructive text-destructive-foreground hover:bg-destructive/90',
};

const sizeStyles: Record<ButtonSize, string> = {
  sm: 'h-8 px-3 text-xs',
  md: 'h-10 px-4 text-sm',
  lg: 'h-12 px-5 text-base',
};

export function Button({ asChild, className, variant = 'primary', size = 'md', ...props }: ButtonProps) {
  const Comp = asChild ? Slot : 'button';
  return (
    <Comp
      className={cn(
        'inline-flex items-center justify-center gap-2 rounded-md border border-border/70 font-semibold transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/70 disabled:cursor-not-allowed disabled:opacity-50',
        variantStyles[variant],
        sizeStyles[size],
        className,
      )}
      {...props}
    />
  );
}
