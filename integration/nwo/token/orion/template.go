/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package orion

const Extension = `
token:
  tms: 
    {{ TMSID }}:
      network: {{ TMS.Network }}
      channel: {{ TMS.Channel }}
      namespace: {{ TMS.Namespace }}
      orion:
        custodian:
          id: {{ CustodianID }}
          enabled: {{ IsCustodian }}
      certification: 
        interactive:
          ids: {{ range TMS.Certifiers }}
          - {{ . }}{{ end }}
      unitydb:
        persistence:
          opts:
            createSchema: true 
            driver: sqlite    
            maxOpenConns: 10
            dataSource: {{ SQLDataSource }}
      ttxdb:
        persistence:
          type: unity
      auditdb:
        persistence:
          type: unity
      tokendb:
        persistence:
          type: sql
          opts:
            createSchema: true 
            tablePrefix: tokens  
            driver: sqlite    
            maxOpenConns: 10
            dataSource: {{ TokensSQLDataSource }}
      {{ if Wallets }}wallets:{{ if Wallets.Certifiers }}
        certifiers: {{ range Wallets.Certifiers }}
        - id: {{ .ID }}
          default: {{ .Default }}
          path: {{ .Path }}
        {{ end }}
      {{ end }}{{ if Wallets.Issuers }}
        issuers: {{ range Wallets.Issuers }}
        - id: {{ .ID }}
          default: {{ .Default }}
          path: {{ .Path }}
        {{ end }}
      {{ end }}{{ if Wallets.Owners }}
        owners: {{ range Wallets.Owners }}
        - id: {{ .ID }}
          default: {{ .Default }}
          path: {{ .Path }}
        {{ end }}
      {{ end }}{{ if Wallets.Auditors }}
        auditors: {{ range Wallets.Auditors }}
        - id: {{ .ID }}
          default: {{ .Default }}
          path: {{ .Path }}
        {{ end }}
      {{ end }}
      {{ end }}
`
