import { Component, type ErrorInfo, type ReactNode } from 'react'
import { ErrorPage } from '@/components/ErrorPage'

interface ErrorBoundaryProps {
  children: ReactNode
  variant?: 'not-found' | 'room-error' | 'kicked' | 'session' | 'server'
  onError?: (error: Error, errorInfo: ErrorInfo) => void
}

interface ErrorBoundaryState {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
  state: ErrorBoundaryState = { hasError: false, error: null }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    console.error('[ErrorBoundary]', error, errorInfo.componentStack)
    this.props.onError?.(error, errorInfo)
  }

  render() {
    if (this.state.hasError) {
      return <ErrorPage variant={this.props.variant ?? 'server'} error={this.state.error?.message} />
    }

    return this.props.children
  }
}
