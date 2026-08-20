import type {JSX} from '@solidjs/web'

import {PropertyProps} from '../types'

import ConfirmPerson from './confirmPerson'

const Person = (props: PropertyProps): JSX.Element => {
    return (
        <ConfirmPerson
            {...props}
            showEmptyPlaceholder={true}
        />
    )
}

export default Person
