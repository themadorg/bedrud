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
          <DialogTitle className="text-white">Share YouTube</DialogTitle>
          <DialogDescription className="text-white/50">
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
          className="border-white/[0.12] bg-white/[0.06] text-white placeholder:text-white/35"
        />

        {error && <p className="text-sm text-red-400">{error}</p>}

        <DialogFooter className="gap-2 sm:gap-0">
          <Button
            type="button"
            variant="ghost"
            onClick={() => closeShareDialog()}
            className="border border-white/10 text-white/60 hover:bg-white/10 hover:text-white"
          >
            Cancel
          </Button>
          <Button type="button" onClick={handleSubmit} disabled={!url.trim()} className="bg-primary text-primary-foreground">
            Share with room
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
