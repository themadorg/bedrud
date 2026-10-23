import { CircleAlert, CircleCheck, Info, Loader2, TriangleAlert } from 'lucide-react'
import { Toaster as Sonner, type ToasterProps } from 'sonner'

/**
 * The app's toaster, matching the Android client's `BedrudSnackbarHost`: every kind shares one
 * inverse bar, and an icon in the inverse status colour says which kind it is. The bar colours, the
 * corner and the error-only close button are set in `styles.css`, because Sonner's own stylesheet
 * is unlayered and outranks Tailwind's utilities.
 *
 * Each icon is named for a screen reader, since the bar no longer tells the kinds apart by colour.
 */
function Toaster(props: ToasterProps) {
  return (
    <Sonner
      closeButton
      icons={{
        success: <CircleCheck role="img" aria-label="Success" className="size-4 text-inverse-success" />,
        info: <Info role="img" aria-label="Information" className="size-4" />,
        warning: <TriangleAlert role="img" aria-label="Warning" className="size-4 text-inverse-warning" />,
        error: <CircleAlert role="img" aria-label="Error" className="size-4 text-inverse-error" />,
        loading: <Loader2 role="img" aria-label="Loading" className="size-4 animate-spin" />,
      }}
      {...props}
    />
  )
}

export { Toaster }
