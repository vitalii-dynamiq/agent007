import { useState } from 'react'
import { 
  Plus, 
  Sparkles, 
  Code, 
  FileText, 
  Play, 
  Trash2, 
  Edit3,
  Check,
  ChevronRight,
  ChevronLeft,
  Zap,
  BookOpen,
  Terminal,
  Database,
  BarChart3,
  GitBranch,
  Shield,
  Clock,
  X,
  Eye,
  Save,
  AlertCircle,
  Copy,
  ToggleLeft,
  ToggleRight
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { cn } from '@/lib/utils'

// Mock skill data
interface Skill {
  id: string
  name: string
  description: string
  icon: string
  category: 'analysis' | 'automation' | 'integration' | 'custom'
  isActive: boolean
  isBuiltIn: boolean
  lastUsed?: string
  usageCount: number
}

const MOCK_SKILLS: Skill[] = [
  {
    id: '1',
    name: 'data-analysis',
    description: 'Analyze datasets using pandas, numpy, and scipy. Create statistical summaries, detect outliers, and identify trends in your data.',
    icon: 'BarChart3',
    category: 'analysis',
    isActive: true,
    isBuiltIn: false,
    lastUsed: '2 hours ago',
    usageCount: 47
  },
  {
    id: '2',
    name: 'sql-query-optimizer',
    description: 'Write and optimize SQL queries for PostgreSQL, MySQL, and BigQuery. Analyze execution plans and suggest index improvements.',
    icon: 'Database',
    category: 'analysis',
    isActive: true,
    isBuiltIn: false,
    lastUsed: '1 day ago',
    usageCount: 23
  },
  {
    id: '3',
    name: 'python-code-review',
    description: 'Review Python code for bugs, security vulnerabilities, and PEP 8 compliance. Suggest refactoring and performance improvements.',
    icon: 'GitBranch',
    category: 'automation',
    isActive: true,
    isBuiltIn: false,
    lastUsed: '5 hours ago',
    usageCount: 31
  },
  {
    id: '4',
    name: 'rest-api-tester',
    description: 'Test REST API endpoints with various HTTP methods. Validate JSON responses, check status codes, and measure response times.',
    icon: 'Terminal',
    category: 'integration',
    isActive: true,
    isBuiltIn: false,
    lastUsed: '3 days ago',
    usageCount: 18
  },
  {
    id: '5',
    name: 'aws-infrastructure',
    description: 'Query and analyze AWS resources using boto3. List EC2 instances, S3 buckets, Lambda functions, and generate cost reports.',
    icon: 'Shield',
    category: 'integration',
    isActive: false,
    isBuiltIn: false,
    usageCount: 9
  },
]

const SKILL_TEMPLATES = [
  {
    id: 'blank',
    name: 'Blank Skill',
    description: 'Start from scratch with a minimal template',
    icon: FileText,
    content: `---
name: my-skill
description: What this skill does and when to use it
---

# Instructions

When this skill is invoked:

1. **First step**: Describe what to do first
2. **Second step**: Describe the next action
3. **Final step**: Complete the task

## Guidelines

- Be specific about expected inputs
- Define clear success criteria
- Handle edge cases gracefully
`
  },
  {
    id: 'data-pipeline',
    name: 'Data Pipeline',
    description: 'Process and transform data with validation steps',
    icon: Database,
    content: `---
name: data-pipeline
description: Process and transform data with validation
---

# Data Pipeline Instructions

## Input Validation
1. Check data format (CSV, JSON, Excel)
2. Validate required columns exist
3. Check for data types and missing values

## Processing Steps
1. Clean data (remove duplicates, handle nulls)
2. Transform columns as needed
3. Apply business logic

## Output
1. Generate summary statistics
2. Export processed data
3. Create validation report
`
  },
  {
    id: 'report-generator',
    name: 'Report Generator',
    description: 'Generate formatted reports from data sources',
    icon: BarChart3,
    content: `---
name: report-generator
description: Generate formatted reports from data
---

# Report Generator

## Data Collection
1. Query the specified data sources
2. Aggregate metrics as needed
3. Calculate derived values

## Report Structure
1. Executive Summary
2. Key Metrics & KPIs
3. Detailed Analysis
4. Visualizations (charts, tables)
5. Recommendations

## Output Formats
- PDF for formal reports
- Excel for data exports
- Markdown for inline display
`
  },
  {
    id: 'workflow-automation',
    name: 'Workflow Automation',
    description: 'Automate multi-step workflows with conditions',
    icon: Zap,
    content: `---
name: workflow-automation
description: Automate multi-step workflows
---

# Workflow Automation

## Trigger Conditions
Define when this workflow should run:
- On specific user request
- When certain data conditions are met

## Steps
1. **Initialize**: Set up required connections
2. **Execute**: Run the main workflow logic
3. **Validate**: Check results meet criteria
4. **Notify**: Report completion status

## Error Handling
- Retry failed steps (max 3 attempts)
- Log errors with context
- Notify user of failures
`
  },
]

const ICON_MAP: Record<string, React.ComponentType<{ className?: string }>> = {
  BarChart3,
  Database,
  GitBranch,
  Terminal,
  Shield,
  Zap,
  FileText,
  Code,
  BookOpen
}

interface SkillEditorProps {
  skill?: Skill | null
  template?: typeof SKILL_TEMPLATES[0] | null
  onSave: (skill: Partial<Skill>) => void
  onCancel: () => void
}

function SkillEditor({ skill, template, onSave, onCancel }: SkillEditorProps) {
  const [name, setName] = useState(skill?.name || template?.name?.replace(/\s+/g, '-').toLowerCase() || '')
  const [description, setDescription] = useState(skill?.description || template?.description || '')
  const [content, setContent] = useState(template?.content || `---
name: ${skill?.name || 'my-skill'}
description: ${skill?.description || 'What this skill does and when to use it'}
---

# Instructions

When this skill is invoked:

1. **First step**: Describe what to do first
2. **Second step**: Describe the next action
3. **Final step**: Complete the task

## Guidelines

- Be specific about expected inputs
- Define clear success criteria
- Handle edge cases gracefully
`)
  const [activeTab, setActiveTab] = useState<'edit' | 'preview'>('preview')
  const [errors, setErrors] = useState<string[]>([])

  const validateSkill = () => {
    const newErrors: string[] = []
    if (!name.trim()) newErrors.push('Skill name is required')
    if (!/^[a-z0-9-]+$/.test(name)) newErrors.push('Name must be lowercase letters, numbers, and hyphens only')
    if (!description.trim()) newErrors.push('Description is required')
    if (content.length < 50) newErrors.push('Skill content is too short')
    setErrors(newErrors)
    return newErrors.length === 0
  }

  const handleSave = () => {
    if (validateSkill()) {
      onSave({ name, description })
    }
  }

  return (
    <div className="flex flex-col h-full bg-background">
      {/* Editor Header */}
      <div className="border-b bg-muted/30">
        <div className="flex items-center justify-between px-4 py-3">
          <div className="flex items-center gap-3">
            <Button 
              variant="ghost" 
              size="icon" 
              onClick={onCancel}
              className="h-8 w-8 cursor-pointer"
            >
              <ChevronLeft className="h-4 w-4" />
            </Button>
            <div className="flex items-center gap-2">
              <div className="p-1.5 rounded-md bg-primary/10">
                <Sparkles className="h-4 w-4 text-primary" />
              </div>
              <div>
                <h3 className="font-semibold text-sm">{skill ? 'Edit Skill' : 'Create Skill'}</h3>
                <p className="text-xs text-muted-foreground">{name || 'untitled'}</p>
              </div>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Button 
              variant="outline" 
              size="sm" 
              onClick={onCancel}
              className="cursor-pointer"
            >
              Cancel
            </Button>
            <Button 
              size="sm" 
              onClick={handleSave}
              className="cursor-pointer"
            >
              <Save className="h-4 w-4 mr-1.5" />
              Save Skill
            </Button>
          </div>
        </div>

        {/* Tab Navigation */}
        <div className="px-4 pb-0">
          <div className="flex gap-0 border-b-0">
            {[
              { id: 'edit', label: 'Edit', icon: Code },
              { id: 'preview', label: 'Preview', icon: Eye },
            ].map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id as 'edit' | 'preview')}
                className={cn(
                  "flex items-center gap-1.5 px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors cursor-pointer",
                  activeTab === tab.id
                    ? "border-primary text-foreground"
                    : "border-transparent text-muted-foreground hover:text-foreground"
                )}
              >
                <tab.icon className="h-3.5 w-3.5" />
                {tab.label}
              </button>
            ))}
          </div>
        </div>
      </div>

      {/* Errors */}
      {errors.length > 0 && (
        <div className="mx-4 mt-4 p-3 rounded-lg bg-destructive/10 border border-destructive/20">
          <div className="flex items-center gap-2 text-destructive text-sm font-medium mb-1">
            <AlertCircle className="h-4 w-4" />
            Please fix the following issues:
          </div>
          <ul className="text-sm text-destructive/80 list-disc list-inside">
            {errors.map((err, i) => <li key={i}>{err}</li>)}
          </ul>
        </div>
      )}

      {/* Editor Content */}
      <div className="flex-1 overflow-hidden">
        {activeTab === 'edit' && (
          <div className="h-full flex flex-col p-4 gap-4">
            {/* Name and Description Row */}
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="text-sm font-medium mb-1.5 block">
                  Skill Name <span className="text-destructive">*</span>
                </label>
                <Input
                  value={name}
                  onChange={(e) => setName(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-'))}
                  placeholder="my-skill-name"
                  className="font-mono text-sm"
                />
                <p className="text-xs text-muted-foreground mt-1">
                  Use lowercase letters, numbers, and hyphens
                </p>
              </div>
              <div>
                <label className="text-sm font-medium mb-1.5 block">
                  Description <span className="text-destructive">*</span>
                </label>
                <Input
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="What this skill does..."
                />
                <p className="text-xs text-muted-foreground mt-1">
                  Helps the agent decide when to use this skill
                </p>
              </div>
            </div>
            
            {/* Editor Area */}
            <div className="flex-1 flex flex-col min-h-0">
              <div className="flex items-center justify-between mb-1.5">
                <label className="text-sm font-medium">SKILL.md Content</label>
                <Button 
                  variant="ghost" 
                  size="sm" 
                  className="h-7 text-xs cursor-pointer"
                  onClick={() => navigator.clipboard.writeText(content)}
                >
                  <Copy className="h-3 w-3 mr-1" />
                  Copy
                </Button>
              </div>
              <div className="flex-1 relative rounded-lg border bg-muted/30 overflow-hidden">
                {/* Line numbers gutter */}
                <div className="absolute left-0 top-0 bottom-0 w-10 bg-muted/50 border-r flex flex-col items-end py-4 pr-2 text-xs text-muted-foreground font-mono select-none overflow-hidden">
                  {content.split('\n').map((_, i) => (
                    <div key={i} className="leading-6">{i + 1}</div>
                  ))}
                </div>
                <textarea
                  value={content}
                  onChange={(e) => setContent(e.target.value)}
                  className={cn(
                    "w-full h-full pl-12 pr-4 py-4 font-mono text-sm leading-6",
                    "bg-transparent focus:outline-none resize-none"
                  )}
                  placeholder="Enter your skill instructions..."
                  spellCheck={false}
                />
              </div>
            </div>
          </div>
        )}

        {activeTab === 'preview' && (
          <ScrollArea className="h-full">
            <div className="p-6 max-w-3xl mx-auto">
              {/* Frontmatter Preview */}
              <div className="bg-muted/30 rounded-lg p-4 mb-6 border">
                <h4 className="text-sm font-medium mb-3 flex items-center gap-2">
                  <FileText className="h-4 w-4" />
                  Skill Metadata
                </h4>
                <div className="grid grid-cols-2 gap-4 text-sm">
                  <div className="space-y-1">
                    <span className="text-muted-foreground text-xs uppercase tracking-wide">Name</span>
                    <p className="font-mono text-primary">{name || 'untitled'}</p>
                  </div>
                  <div className="space-y-1">
                    <span className="text-muted-foreground text-xs uppercase tracking-wide">Description</span>
                    <p>{description || 'No description'}</p>
                  </div>
                </div>
              </div>
              
              {/* Content Preview */}
              <div className="prose prose-sm dark:prose-invert max-w-none">
                <h4 className="text-sm font-medium mb-3 flex items-center gap-2 not-prose">
                  <BookOpen className="h-4 w-4" />
                  Instructions Preview
                </h4>
                <div className="bg-background rounded-lg border p-6">
                  <pre className="whitespace-pre-wrap text-sm font-sans">
                    {content.split('---').slice(2).join('---').trim() || 'No content yet...'}
                  </pre>
                </div>
              </div>
            </div>
          </ScrollArea>
        )}

      </div>
    </div>
  )
}

function SkillCard({ skill, onEdit, onToggle, onDelete }: { 
  skill: Skill
  onEdit: () => void
  onToggle: () => void
  onDelete: () => void
}) {
  const IconComponent = ICON_MAP[skill.icon] || Sparkles

  return (
    <div className={cn(
      "group relative rounded-lg border p-4 transition-all hover:shadow-md cursor-pointer",
      skill.isActive ? "bg-background" : "bg-muted/30 opacity-75"
    )}
    onClick={onEdit}
    >
      <div className="flex items-start gap-3">
        <div className={cn(
          "p-2 rounded-lg",
          skill.isActive ? "bg-primary/10" : "bg-muted"
        )}>
          <IconComponent className={cn(
            "h-5 w-5",
            skill.isActive ? "text-primary" : "text-muted-foreground"
          )} />
        </div>
        <div className="flex-1 min-w-0 pr-20">
          <div className="flex items-center gap-2">
            <h4 className="font-medium text-sm truncate">{skill.name}</h4>
          </div>
          <p className="text-xs text-muted-foreground mt-1 line-clamp-2">
            {skill.description}
          </p>
          <div className="flex items-center gap-3 mt-2 text-xs text-muted-foreground">
            {skill.lastUsed && (
              <span className="flex items-center gap-1">
                <Clock className="h-3 w-3" />
                {skill.lastUsed}
              </span>
            )}
            <span className="flex items-center gap-1">
              <Play className="h-3 w-3" />
              {skill.usageCount} uses
            </span>
          </div>
        </div>
      </div>

      {/* Actions - always visible on right side */}
      <div className="absolute top-3 right-3 flex items-center gap-1">
        <Button 
          variant="ghost" 
          size="icon" 
          className="h-8 w-8 cursor-pointer opacity-60 hover:opacity-100" 
          onClick={(e) => { e.stopPropagation(); onEdit() }}
          title="Edit skill"
        >
          <Edit3 className="h-4 w-4" />
        </Button>
        <Button 
          variant="ghost" 
          size="icon" 
          className="h-8 w-8 cursor-pointer opacity-60 hover:opacity-100" 
          onClick={(e) => { e.stopPropagation(); onToggle() }}
          title={skill.isActive ? "Deactivate skill" : "Activate skill"}
        >
          {skill.isActive ? (
            <ToggleRight className="h-4 w-4 text-green-500" />
          ) : (
            <ToggleLeft className="h-4 w-4" />
          )}
        </Button>
        <Button 
          variant="ghost" 
          size="icon" 
          className="h-8 w-8 cursor-pointer text-muted-foreground hover:text-destructive opacity-60 hover:opacity-100" 
          onClick={(e) => { e.stopPropagation(); onDelete() }}
          title="Delete skill"
        >
          <Trash2 className="h-4 w-4" />
        </Button>
      </div>
    </div>
  )
}

export function SkillsPanel() {
  const [skills, setSkills] = useState<Skill[]>(MOCK_SKILLS)
  const [searchQuery, setSearchQuery] = useState('')
  const [selectedCategory, setSelectedCategory] = useState<string | null>(null)
  const [isCreating, setIsCreating] = useState(false)
  const [editingSkill, setEditingSkill] = useState<Skill | null>(null)
  const [selectedTemplate, setSelectedTemplate] = useState<typeof SKILL_TEMPLATES[0] | null>(null)
  const [showTemplates, setShowTemplates] = useState(false)

  const filteredSkills = skills.filter(skill => {
    const matchesSearch = skill.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
                         skill.description.toLowerCase().includes(searchQuery.toLowerCase())
    const matchesCategory = !selectedCategory || skill.category === selectedCategory
    return matchesSearch && matchesCategory
  })

  const categories = [
    { id: 'analysis', label: 'Analysis', icon: BarChart3 },
    { id: 'automation', label: 'Automation', icon: Zap },
    { id: 'integration', label: 'Integration', icon: GitBranch },
    { id: 'custom', label: 'Custom', icon: Code },
  ]

  if (isCreating || editingSkill) {
    return (
      <SkillEditor
        skill={editingSkill}
        template={selectedTemplate}
        onSave={() => {
          setIsCreating(false)
          setEditingSkill(null)
          setSelectedTemplate(null)
        }}
        onCancel={() => {
          setIsCreating(false)
          setEditingSkill(null)
          setSelectedTemplate(null)
        }}
      />
    )
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="border-b p-4">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-gradient-to-br from-purple-500/20 to-pink-500/20">
              <Sparkles className="h-5 w-5 text-purple-500" />
            </div>
            <div>
              <h2 className="font-semibold">Agent Skills</h2>
              <p className="text-xs text-muted-foreground">
                Extend your agent's capabilities with custom instructions
              </p>
            </div>
          </div>
          <Button size="sm" onClick={() => setShowTemplates(true)} className="cursor-pointer">
            <Plus className="h-4 w-4 mr-1" />
            New Skill
          </Button>
        </div>

        {/* Search */}
        <Input
          placeholder="Search skills..."
          value={searchQuery}
          onChange={(e) => setSearchQuery(e.target.value)}
          className="mb-3"
        />

        {/* Category Filter */}
        <div className="flex gap-2 flex-wrap">
          <Button
            variant={selectedCategory === null ? "secondary" : "ghost"}
            size="sm"
            onClick={() => setSelectedCategory(null)}
            className="h-8 cursor-pointer"
          >
            All
          </Button>
          {categories.map(cat => (
            <Button
              key={cat.id}
              variant={selectedCategory === cat.id ? "secondary" : "ghost"}
              size="sm"
              onClick={() => setSelectedCategory(cat.id)}
              className="h-8 cursor-pointer"
            >
              <cat.icon className="h-3.5 w-3.5 mr-1.5" />
              {cat.label}
            </Button>
          ))}
        </div>
      </div>

      {/* Skills List */}
      <ScrollArea className="flex-1">
        <div className="p-4 grid gap-3">
          {filteredSkills.length === 0 ? (
            <div className="text-center py-12">
              <Sparkles className="h-12 w-12 mx-auto text-muted-foreground/30 mb-3" />
              <p className="text-sm text-muted-foreground">No skills found</p>
              <Button 
                variant="link" 
                size="sm" 
                className="mt-2 cursor-pointer"
                onClick={() => setShowTemplates(true)}
              >
                Create your first skill
              </Button>
            </div>
          ) : (
            filteredSkills.map(skill => (
              <SkillCard
                key={skill.id}
                skill={skill}
                onEdit={() => setEditingSkill(skill)}
                onToggle={() => {
                  setSkills(prev => prev.map(s => 
                    s.id === skill.id ? { ...s, isActive: !s.isActive } : s
                  ))
                }}
                onDelete={() => {
                  setSkills(prev => prev.filter(s => s.id !== skill.id))
                }}
              />
            ))
          )}
        </div>
      </ScrollArea>

      {/* Info Footer */}
      <div className="border-t p-3 bg-muted/30">
        <div className="flex items-center justify-between text-xs text-muted-foreground">
          <span>{skills.filter(s => s.isActive).length} active skills</span>
          <a 
            href="https://agentskills.io" 
            target="_blank" 
            rel="noopener noreferrer"
            className="flex items-center gap-1 hover:text-foreground transition-colors cursor-pointer"
          >
            Learn more about Agent Skills
            <ChevronRight className="h-3 w-3" />
          </a>
        </div>
      </div>

      {/* Template Selection Modal */}
      {showTemplates && (
        <div className="absolute inset-0 bg-background/95 backdrop-blur-sm flex items-center justify-center p-6 z-10">
          <div className="w-full max-w-2xl">
            <div className="flex items-center justify-between mb-6">
              <div>
                <h3 className="text-lg font-semibold">Choose a Template</h3>
                <p className="text-sm text-muted-foreground">Start with a template or create from scratch</p>
              </div>
              <Button 
                variant="ghost" 
                size="icon" 
                onClick={() => setShowTemplates(false)}
                className="cursor-pointer"
              >
                <X className="h-4 w-4" />
              </Button>
            </div>
            <div className="grid grid-cols-2 gap-4">
              {SKILL_TEMPLATES.map(template => (
                <button
                  key={template.id}
                  onClick={() => {
                    setSelectedTemplate(template)
                    setShowTemplates(false)
                    setIsCreating(true)
                  }}
                  className={cn(
                    "flex items-start gap-4 p-5 rounded-xl border bg-background text-left cursor-pointer",
                    "hover:border-primary hover:shadow-lg transition-all duration-200",
                    "focus:outline-none focus:ring-2 focus:ring-primary focus:ring-offset-2"
                  )}
                >
                  <div className="p-2.5 rounded-lg bg-primary/10 shrink-0">
                    <template.icon className="h-5 w-5 text-primary" />
                  </div>
                  <div>
                    <h4 className="font-semibold text-sm">{template.name}</h4>
                    <p className="text-xs text-muted-foreground mt-1 leading-relaxed">
                      {template.description}
                    </p>
                  </div>
                </button>
              ))}
            </div>
            <div className="mt-6 pt-4 border-t text-center">
              <Button 
                variant="ghost" 
                onClick={() => setShowTemplates(false)}
                className="cursor-pointer"
              >
                Cancel
              </Button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
