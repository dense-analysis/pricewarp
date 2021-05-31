import React, {Component} from 'react'

import {Container} from '@material-ui/core'

import {WbNav} from './components/WbNav'

interface AppProps {
}

interface AppState {
}

/**
 * The pricewarp app.
 */
export class App extends Component<AppProps, AppState> {
  render() {
    return <Container>
      <Container>
        <WbNav />
      </Container>
    </Container>
  }
}
