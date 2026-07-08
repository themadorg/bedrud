import { useEffect, useState } from 'react'
import { resolveAvatarUrl } from '#/lib/avatar-url'
import { cn } from '#/lib/utils'

interface Props {
  avatarUrl?: string
  initials: string
  paletteBackground: string
  className?: string
  textClassName?: string
  style?: React.CSSProperties
}

export function ParticipantAvatar({ avatarUrl, initials, paletteBackground, className, textClassName, style }: Props) {
  const resolved = resolveAvatarUrl(avatarUrl)
  const [imageError, setImageError] = useState(false)

  useEffect(() => {
    setImageError(false)
  }, [resolved])

  const showImage = Boolean(resolved) && !imageError

  return (
    <div
      className={cn('meet-avatar-circle flex items-center justify-center overflow-hidden font-bold text-white', className)}
      style={{
        ...style,
        background: paletteBackground,
        clipPath: 'circle(50% at 50% 50%)',
      }}
    >
      {showImage ? (
        <img
          key={resolved}
          src={resolved}
          alt=""
          className="h-full w-full object-cover"
          referrerPolicy="no-referrer"
          onError={() => setImageError(true)}
        />
      ) : (
        <span className={textClassName}>{initials}</span>
      )}
    </div>
  )
}
