export interface AccountErrorStats {
  coverage_start: string
  coverage_end: string
  partial: boolean
  uncorrelated_ws_errors: number
  uncorrelated_errors?: number // Includes WS and policy failures without reliable request correlation.
  total_requests: number
  error_count: number
  error_rate: number | null // Percentage (0–100); null without a reliable denominator.
  history: {
    date: string
    label: string
    requests: number
    error_count: number
    error_rate: number | null
  }[]
}
