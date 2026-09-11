// Theme selection. Three states, not two: "auto" follows the operating system,
// which is what most people actually want — their machine already switches at
// sunset and the app should not fight that.
//
// The resolved theme is written to <html data-theme>, so CSS variables switch
// without React re-rendering anything.

export type ThemeChoice = 'auto' | 'light' | 'dark'

const KEY = 'pulse.theme'
const media = window.matchMedia('(prefers-color-scheme: dark)')

export function getChoice(): ThemeChoice {
  const v = localStorage.getItem(KEY)
  return v === 'light' || v === 'dark' ? v : 'auto'
}

export function resolve(choice: ThemeChoice): 'light' | 'dark' {
  if (choice === 'auto') return media.matches ? 'dark' : 'light'
  return choice
}

export function apply(choice: ThemeChoice) {
  document.documentElement.dataset.theme = resolve(choice)
}

export function setChoice(choice: ThemeChoice) {
  if (choice === 'auto') localStorage.removeItem(KEY)
  else localStorage.setItem(KEY, choice)
  apply(choice)
}

// Called once at startup, before React renders, so there is no flash of the
// wrong theme on load.
export function init() {
  apply(getChoice())
  // Following the system means reacting when the system changes, not just
  // reading it once — otherwise "auto" is only correct until sunset.
  media.addEventListener('change', () => {
    if (getChoice() === 'auto') apply('auto')
  })
}
