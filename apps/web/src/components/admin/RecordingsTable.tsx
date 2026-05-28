// TODO oncoming feature
export interface RecordingItem {
  id: string
  recordingType: string
  durationMs: number
  fileSize: number
  fileUrl?: string
  status: string
  error?: string
  downloadStatus: 'processing' | 'ready' | 'failed'
  roomId: string
  roomName: string
  createdBy: string
  createdAt: string
}

export function RecordingsTable() {
  return <div className="text-center text-muted-foreground py-8">Recordings coming in a future release</div>
}
