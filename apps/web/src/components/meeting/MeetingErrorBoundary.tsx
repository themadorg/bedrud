import { Component, type ErrorInfo, type ReactNode } from 'react'
import { ErrorPage } from '@/components/ErrorPage'

interface Props {
  children: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export class MeetingErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false, error: null }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    if (import.meta.env.DEV) {
      console.error('[MeetingErrorBoundary]', error, info.componentStack)
    }
  }

  render() {
    if (this.state.hasError) {
      return (
        <ErrorPage
          variant="server"
          title="Meeting error"
          description="Something went wrong inside the meeting. Try reloading the page."
          error={this.state.error?.message}
          showBack={false}
        />
      )
    }
    return this.props.children
  }
}
