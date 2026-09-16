import type {JSX} from '@solidjs/web'

type Props = {
    icon: string
    class?: string
}

export default function CompassIcon(props: Props): JSX.Element {
    // All compass icon classes start with icon,
    // so not expecting that prefix in props.
    return (
        <i class={`CompassIcon icon-${props.icon}${props.class === undefined ? '' : ` ${props.class}`}`}/>
    )
}
