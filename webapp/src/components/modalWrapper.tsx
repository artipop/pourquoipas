import type {JSX} from '@solidjs/web'

import './modalWrapper.scss'

type Props = {
    children: JSX.Element
}

const ModalWrapper = (props: Props) => {
    return (
        <div class='ModalWrapper'>
            {props.children}
        </div>
    )
}

export default ModalWrapper
