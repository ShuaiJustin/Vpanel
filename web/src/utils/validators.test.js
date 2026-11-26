import { describe, expect, it } from 'vitest'
import { isIP } from 'node:net'
import { isValidIPv4Address, isValidIPv6Address, ipAddress } from './validators'

describe('IP address validation', () => {
  it.each([
    '2607:5501:3000:977::2', '::1', '::', '2001:db8::', '2001:db8:0:1::5',
    '2001:DB8:0000:0000:0000:0000:0000:0001', '::ffff:192.0.2.1',
    '192.0.2.1', '0.0.0.0', '255.255.255.255',
    '999.1.1.1', '01.2.3.4', '1.2.3', '2001:db8:::2', '2001::db8::2',
    '2001:db8:1:2:3:4:5:6:7', '2001:db8:1:2:3:4:5', '12345::1',
    '[2607:5501:3000:977::2]', '::ffff:999.0.0.1',
    'example.com', 'http://[::1]', '',
  ])('agrees with the native IP parser for %s', value => {
    expect(isValidIPv4Address(value)).toBe(isIP(value) === 4)
    expect(isValidIPv6Address(value)).toBe(isIP(value) === 6)
  })

  it('accepts the reported compressed IPv6 through the form rule', () => {
    let error
    ipAddress().validator({}, '2607:5501:3000:977::2', e => { error = e })
    expect(error).toBeUndefined()
    ipAddress().validator({}, '2607::5501::2', e => { error = e })
    expect(error).toBeInstanceOf(Error)
  })

  it('rejects scoped addresses unsupported by the backend address parser', () => {
    expect(isValidIPv6Address('fe80::1%eth0')).toBe(false)
  })
})
