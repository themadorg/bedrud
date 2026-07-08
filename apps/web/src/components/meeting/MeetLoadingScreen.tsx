interface MeetLoadingScreenProps {
  label?: string
}

export function MeetLoadingScreen({ label }: MeetLoadingScreenProps) {
  return (
    <div className="meet-room meet-loading-screen">
      <div className="meet-loading-spinner" aria-hidden />
      {label ? <p className="meet-loading-label">{label}</p> : null}
    </div>
  )
}
