import { useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { useYoutubeWatch } from './youtube-watch-context'

export function YoutubeShareDialog() {
  const { shareDialogOpen, closeShareDialog, shareVideo } = useYoutubeWatch()
  const [url, setUrl] = useState('')
  const [error, setError] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  const handleSubmit = () => {
    const err = shareVideo(url)
    if (err) {
      setError(err)
      return
    }
    setUrl('')
    setError(null)
  }

  return (
    <Dialog
      open={shareDialogOpen}
      onOpenChange={(open) => {
        if (!open) {
          closeShareDialog()
          setError(null)
        }
      }}
    >
      <DialogContent
        className="meet-dialog sm:max-w-md"
        onOpenAutoFocus={(event) => {
          event.preventDefault()
          inputRef.current?.focus()
        }}
      >
        <DialogHeader>
          <DialogTitle className="text-[var(--meet-fg-strong)]">Share YouTube</DialogTitle>
          <DialogDescription className="text-[var(--meet-fg-muted)]">
            Paste a YouTube link. Everyone in the room will watch together in sync.
          </DialogDescription>
        </DialogHeader>

        <Input
          ref={inputRef}
          value={url}
          onChange={(e) => {
            setUrl(e.target.value)
            if (error) setError(null)
          }}
          onKeyDown={(e) => {
            if (e.key === 'Enter') handleSubmit()
          }}
          placeholder="https://youtube.com/watch?v=..."
          className="rounded-lg border border-[var(--meet-border)] bg-[var(--meet-control)] px-2.5 text-[var(--meet-fg-strong)] placeholder:text-[var(--meet-fg-subtle)] focus-visible:border-[color-mix(in_oklab,var(--meet-accent)_40%,transparent)]"
        />

        {error && <p className="text-sm text-destructive">{error}</p>}

        <DialogFooter className="gap-2 sm:gap-0">
          <Button
            type="button"
            variant="ghost"
            onClick={() => closeShareDialog()}
            className="border border-[var(--meet-border-subtle)] text-[var(--meet-fg-muted)] hover:bg-[var(--meet-control)] hover:text-[var(--meet-fg-strong)]"
          >
            Cancel
          </Button>
          <Button
            type="button"
            onClick={handleSubmit}
            disabled={!url.trim()}
            className="bg-primary text-primary-foreground"
          >
            Share with room
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
