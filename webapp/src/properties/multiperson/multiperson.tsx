import type {JSX} from '@solidjs/web'

import {PropertyProps} from '../types'
import ConfirmPerson from '../person/confirmPerson'

const MultiPerson = (props: PropertyProps): JSX.Element => {
    return (
        <ConfirmPerson
            {...props}
            showEmptyPlaceholder={true}
        />
    )
}

export default MultiPerson
