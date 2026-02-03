/* eslint-disable react-refresh/only-export-components */
/**
 * Brand icons for integrations using react-icons
 * 
 * Documentation: https://react-icons.github.io/react-icons/
 * 
 * Icon sources:
 * - Si* = Simple Icons (brand logos)
 * - Fa* = Font Awesome
 * - Vsc* = VS Code Icons
 */

import {
  SiGithub,
  SiSlack,
  SiNotion,
  SiGmail,
  SiGooglecalendar,
  SiGoogledrive,
  SiLinear,
  SiJira,
  SiConfluence,
  SiAsana,
  SiTrello,
  SiAirtable,
  SiDiscord,
  SiStripe,
  SiVercel,
  SiSupabase,
  SiCloudflare,
  SiDatadog,
  SiNewrelic,
  SiPagerduty,
  SiSentry,
  SiHubspot,
  SiCanva,
  SiAmazonwebservices,
  SiGooglecloud,
  SiKubernetes,
  SiTerraform,
  SiClickup,
  SiOracle,
  SiSnowflake,
  SiDatabricks,
  SiPostgresql,
  SiMysql,
  SiGooglebigquery,
} from 'react-icons/si'

import { 
  FaDatabase, 
  FaCloud, 
  FaFire, 
  FaCalendar,
  FaEnvelope,
  FaUsers,
  FaMicrosoft,
} from 'react-icons/fa'

import { VscAzure } from 'react-icons/vsc'

import type { CSSProperties } from 'react'
import type { IconType } from 'react-icons'

// Map integration IDs to their icons
const integrationIcons: Record<string, IconType> = {
  // Developer Tools
  github: SiGithub,
  stripe: SiStripe,
  vercel: SiVercel,
  supabase: SiSupabase,
  neon: FaDatabase,
  cloudflare: SiCloudflare,
  sentry: SiSentry,
  linear: SiLinear,
  jira: SiJira,
  confluence: SiConfluence,
  clickup: SiClickup,
  
  // Cloud Providers
  aws: SiAmazonwebservices,
  gcp: SiGooglecloud,
  azure: VscAzure,
  ibm_cloud: FaCloud,
  oracle_cloud: SiOracle,
  kubernetes: SiKubernetes,
  
  // Productivity & File Storage
  gmail: SiGmail,
  google_calendar: SiGooglecalendar,
  google_drive: SiGoogledrive,
  onedrive: FaMicrosoft,
  sharepoint: FaMicrosoft,
  notion: SiNotion,
  asana: SiAsana,
  monday: FaCalendar,
  trello: SiTrello,
  airtable: SiAirtable,
  hubspot: SiHubspot,
  
  // Communication
  slack: SiSlack,
  discord: SiDiscord,
  microsoft_teams: FaUsers,
  outlook: FaEnvelope,
  
  // Monitoring
  datadog: SiDatadog,
  newrelic: SiNewrelic,
  pagerduty: SiPagerduty,
  
  // Data Warehouses & Databases
  snowflake: SiSnowflake,
  databricks: SiDatabricks,
  postgres: SiPostgresql,
  mysql: SiMysql,
  bigquery: SiGooglebigquery,
  sqlserver: FaDatabase,
  vertica: FaDatabase,
  
  // Other
  fireflies: FaFire,
  canva: SiCanva,
  terraform: SiTerraform,
}

// Brand colors for icons
const integrationColors: Record<string, string> = {
  github: '#181717',
  slack: '#4A154B',
  notion: '#000000',
  gmail: '#EA4335',
  google_calendar: '#4285F4',
  google_drive: '#4285F4',
  onedrive: '#0078D4',
  sharepoint: '#038387',
  linear: '#5E6AD2',
  jira: '#0052CC',
  confluence: '#172B4D',
  asana: '#F06A6A',
  trello: '#0052CC',
  airtable: '#18BFFF',
  discord: '#5865F2',
  stripe: '#635BFF',
  vercel: '#000000',
  supabase: '#3ECF8E',
  cloudflare: '#F38020',
  datadog: '#632CA6',
  newrelic: '#008C99',
  pagerduty: '#06AC38',
  sentry: '#362D59',
  hubspot: '#FF7A59',
  canva: '#00C4CC',
  aws: '#FF9900',
  gcp: '#4285F4',
  azure: '#0078D4',
  ibm_cloud: '#054ADA',
  oracle_cloud: '#F80000',
  kubernetes: '#326CE5',
  microsoft_teams: '#6264A7',
  outlook: '#0078D4',
  clickup: '#7B68EE',
  neon: '#00E699',
  fireflies: '#FF6B35',
  terraform: '#7B42BC',
  monday: '#FF3D57',
  snowflake: '#29B5E8',
  databricks: '#FF3621',
  postgres: '#4169E1',
  mysql: '#4479A1',
  bigquery: '#669DF6',
  sqlserver: '#CC2927',
  vertica: '#0073C6',
}

// Default icon for unknown integrations
const DefaultIcon = FaCloud

interface IntegrationIconProps {
  id: string
  className?: string
  style?: CSSProperties
}

export function IntegrationIcon({ id, className, style }: IntegrationIconProps) {
  const Icon = integrationIcons[id] || DefaultIcon
  return <Icon className={className} style={style} />
}

// Get color for an integration
export function getIntegrationColor(id: string): string {
  return integrationColors[id] || '#6B7280'
}
