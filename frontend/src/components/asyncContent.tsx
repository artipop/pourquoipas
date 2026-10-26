import {Errored, Loading} from 'solid-js'
import type {JSX} from '@solidjs/web'

import {FormattedMessage} from '../intl'

export const LoadingMessage = () => (
    <p role='status'>
        <FormattedMessage id='AsyncContent.loading' defaultMessage='Loading…'/>
    </p>
)

export const LoadError = (props: {onRetry: () => void}) => (
    <div role='alert'>
        <p>
            <FormattedMessage id='AsyncContent.failed' defaultMessage='Could not load this content.'/>
        </p>
        <button
            type='button'
            onClick={(event) => {
                event.stopPropagation()
                props.onRetry()
            }}
        >
            <FormattedMessage id='AsyncContent.retry' defaultMessage='Try again'/>
        </button>
    </div>
)

// Keep the surrounding screen usable while a local chunk is in flight or
// unavailable. Reset remounts the branch so lazy() can retry a rejected load.
export const AsyncContent = (props: {children: JSX.Element, fallback?: JSX.Element}) => (
    <Errored fallback={(_error, reset) => <LoadError onRetry={reset}/>}>
        <Loading fallback={'fallback' in props ? props.fallback : <LoadingMessage/>}>
            {props.children}
        </Loading>
    </Errored>
)
