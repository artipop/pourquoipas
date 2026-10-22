import {Show, createSignal} from 'solid-js'

import {useLocation, useNavigate} from '@solidjs/router'

import {Redirect} from '../route'

import {FormattedMessage, useIntl} from '../intl'

import {useAppSelector, useAppStore} from '../store/hooks'
import {getLoggedIn} from '../store/users'
import {getRegistrationOpen} from '../store/clientConfig'

import Button from '../widgets/buttons/button'
import client from '../octoClient'
import {readSettings} from '../settingsRead'
import './loginPage.scss'

const LoginPage = () => {
    const intl = useIntl()
    const [username, setUsername] = createSignal('')
    const [password, setPassword] = createSignal('')
    const [errorMessage, setErrorMessage] = createSignal('')
    const {actions} = useAppStore()
    const loggedIn = useAppSelector<boolean|null>(getLoggedIn)

    // A server nobody has registered on has no account to log in with, and the
    // only way in is a URL you would have to know. So the login page is the one
    // place that knows to step aside: the first person through the door
    // registers, and registering is what makes them the administrator.
    const registrationOpen = useAppSelector<boolean>(getRegistrationOpen)
    const queryParams = new URLSearchParams(useLocation().search)
    const navigate = useNavigate()

    const handleLogin = async (): Promise<void> => {
        const logged = await client.login(username(), password())
        if (logged) {
            await Promise.all([actions.users.fetchMe(), readSettings()])
            actions.language.fetchLanguage()
            if (queryParams) {
                navigate(queryParams.get('r') || '/')
            } else {
                navigate('/')
            }
        } else {
            setErrorMessage(intl.formatMessage({id: 'login.failed', defaultMessage: 'Login failed'}))
        }
    }

    return (
        <Show
            when={!loggedIn()}
            fallback={<Redirect href={'/'}/>}
        >
            <Show when={registrationOpen()}>
                <Redirect href={'/register'}/>
            </Show>
            <div class='LoginPage'>
                <form
                    onSubmit={(e: Event) => {
                        e.preventDefault()
                        handleLogin()
                    }}
                >
                    <div class='title'>
                        <FormattedMessage
                            id='login.log-in-title'
                            defaultMessage='Log in'
                        />
                    </div>
                    <div class='username'>
                        <input
                            id='login-username'
                            placeholder={intl.formatMessage({id: 'login.username-placeholder', defaultMessage: 'Enter username'})}
                            value={username()}
                            onInput={(e) => {
                                setUsername(e.target.value)
                                setErrorMessage('')
                            }}
                        />
                    </div>
                    <div class='password'>
                        <input
                            id='login-password'
                            type='password'
                            placeholder={intl.formatMessage({id: 'login.password-placeholder', defaultMessage: 'Enter password'})}
                            value={password()}
                            onInput={(e) => {
                                setPassword(e.target.value)
                                setErrorMessage('')
                            }}
                        />
                    </div>
                    <Button
                        filled={true}
                        submit={true}
                    >
                        <FormattedMessage
                            id='login.log-in-button'
                            defaultMessage='Log in'
                        />
                    </Button>
                </form>
                <a href='/register'>
                    <FormattedMessage
                        id='login.register-button'
                        defaultMessage={'or create an account if you don\'t have one'}
                    />
                </a>
                <Show when={errorMessage()}>
                    <div class='error'>
                        {errorMessage()}
                    </div>
                </Show>
            </div>
        </Show>
    )
}

export default LoginPage
