import type {LocaleInput} from '@fullcalendar/core'

let locales: Promise<LocaleInput[]> | undefined

export function loadCalendarLocales(): Promise<LocaleInput[]> {
    if (!locales) {
        locales = import('@fullcalendar/core/locales-all').then((module) => module.default).catch((error) => {
            locales = undefined
            throw error
        })
    }
    return locales
}

// Locale definitions are needed when the calendar first paints. Discover
// both imports at the same time instead of after the component has mounted.
export async function loadCalendar(locale: string) {
    const [module] = await Promise.all([
        import('./fullCalendar'),
        locale.toLowerCase() === 'en' ? undefined : loadCalendarLocales(),
    ])
    return module
}
