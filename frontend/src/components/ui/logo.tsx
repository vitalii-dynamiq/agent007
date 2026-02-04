/**
 * Dynamiq Logo Component
 */

import { cn } from '@/lib/utils'

interface LogoProps {
  className?: string
  size?: 'sm' | 'md' | 'lg'
}

export function DynamiqLogo({ className, size = 'md' }: LogoProps) {
  const sizes = {
    sm: { width: 20, height: 23 },
    md: { width: 26, height: 30 },
    lg: { width: 31, height: 35 },
  }

  const { width, height } = sizes[size]

  return (
    <svg 
      xmlns="http://www.w3.org/2000/svg" 
      width={width} 
      height={height} 
      viewBox="0 0 31 35" 
      fill="none"
      className={cn(className)}
    >
      <path 
        fillRule="evenodd" 
        clipRule="evenodd" 
        d="M13.2366 1.00107L11.1921 0.309082V2.46754V10.4528L2.04452 7.35673L0 6.66473V8.82319V26.4315V27.5102L1.01193 27.8839L17.3625 33.921L19.447 34.6907V32.4687V24.2025L28.5546 27.5654L30.639 28.335V26.113V8.00159V6.89113L29.5872 6.53513L13.2366 1.00107ZM19.447 20.9019L27.5427 23.8911V10.6753L19.447 15.2702V20.9019ZM17.7506 12.6726L14.2884 11.5008V4.62599L25.3385 8.366L17.7506 12.6726ZM14.2884 14.7697L16.3506 15.4677V19.7586L14.2884 18.9972V14.7697ZM11.1921 13.7217V19.1481L3.09636 23.743V10.9816L11.1921 13.7217ZM5.1784 26.1216L12.8507 21.767L16.3506 23.0593V30.2467L5.1784 26.1216Z" 
        fill="currentColor"
      />
    </svg>
  )
}
