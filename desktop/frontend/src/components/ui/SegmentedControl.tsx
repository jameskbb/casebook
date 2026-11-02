// SegmentedControl (UI-SPEC Component Contract): a radio group of 36px
// segments used for the confidence picker, the relation picker and the theme
// switch. Roving tabindex: the group is one Tab stop; Left and Right (and Up
// and Down) move and select, wrapping at the ends.
import { useId, useRef, type KeyboardEvent } from 'react'
import { CircleAlert, type LucideIcon } from 'lucide-react'
import './Field.css'
import './SegmentedControl.css'

export interface SegmentedOption<T extends string> {
  value: T
  label: string
  icon?: LucideIcon
}

export interface SegmentedControlProps<T extends string> {
  /** Visible label above the control (and the group's accessible name). */
  label: string
  /** Keep the label for assistive technology only. */
  hideLabel?: boolean
  options: SegmentedOption<T>[]
  value: T
  onChange: (value: T) => void
  error?: string
  disabled?: boolean
  className?: string
  /** Show only the icons; each label stays as the accessible name and tooltip. */
  iconOnly?: boolean
}

export function SegmentedControl<T extends string>({
  label,
  hideLabel = false,
  options,
  value,
  onChange,
  error,
  disabled = false,
  className,
  iconOnly = false,
}: SegmentedControlProps<T>) {
  const base = useId()
  const labelId = `${base}-label`
  const errorId = `${base}-error`
  const refs = useRef<(HTMLButtonElement | null)[]>([])
  const selected = Math.max(
    0,
    options.findIndex((o) => o.value === value),
  )

  const move = (delta: number) => {
    const next = (selected + delta + options.length) % options.length
    onChange(options[next].value)
    refs.current[next]?.focus()
  }

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (disabled) return
    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
      e.preventDefault()
      move(1)
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
      e.preventDefault()
      move(-1)
    }
  }

  return (
    <div className={['segmented-field', error ? 'is-invalid' : '', className ?? ''].filter(Boolean).join(' ')}>
      <span id={labelId} className={hideLabel ? 'sr-only' : 'field-label'}>
        {label}
      </span>
      <div
        className="segmented"
        role="radiogroup"
        aria-labelledby={labelId}
        aria-invalid={error ? true : undefined}
        aria-describedby={error ? errorId : undefined}
        aria-disabled={disabled || undefined}
        onKeyDown={onKeyDown}
      >
        {options.map((o, i) => {
          const checked = i === selected
          const Icon = o.icon
          return (
            <button
              key={o.value}
              ref={(el) => {
                refs.current[i] = el
              }}
              type="button"
              role="radio"
              aria-checked={checked}
              tabIndex={checked ? 0 : -1}
              disabled={disabled}
              title={iconOnly ? o.label : undefined}
              className={['segment', checked ? 'is-selected' : '', iconOnly && Icon ? 'is-icon-only' : '']
                .filter(Boolean)
                .join(' ')}
              onClick={() => onChange(o.value)}
            >
              {Icon && <Icon size={16} strokeWidth={1.75} aria-hidden="true" />}
              <span className={iconOnly && Icon ? 'sr-only' : undefined}>{o.label}</span>
            </button>
          )
        })}
      </div>
      {error && (
        <p className="field-error" id={errorId}>
          <CircleAlert size={16} strokeWidth={1.75} aria-hidden="true" />
          <span>{error}</span>
        </p>
      )}
    </div>
  )
}
