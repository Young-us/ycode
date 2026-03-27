---
name: frontend-design
description: UI/UX design specialist for creating beautiful, responsive interfaces. Use when designing components, layouts, or styling.
triggers:
  - design
  - ui
  - ux
  - frontend
  - component
  - layout
commands:
  - /design
---

# Frontend Design Guidelines

## Design Principles

1. **Consistency**
   - Use consistent spacing (8px grid system)
   - Maintain consistent color palette
   - Follow typography hierarchy

2. **Accessibility**
   - Ensure sufficient color contrast (WCAG AA)
   - Provide proper focus states
   - Include ARIA labels where needed
   - Support keyboard navigation

3. **Responsiveness**
   - Mobile-first approach
   - Use flexible layouts (Flexbox/Grid)
   - Test on multiple screen sizes

## Component Structure

### Recommended Pattern
```
Component/
├── Component.tsx        # Main component
├── Component.test.tsx   # Tests
├── Component.stories.tsx # Storybook
├── styles.module.css   # Styles
└── index.ts           # Export
```

### Props Interface
```typescript
interface ComponentProps {
  // Required props
  children: React.ReactNode;
  
  // Optional props
  variant?: 'primary' | 'secondary';
  size?: 'sm' | 'md' | 'lg';
  className?: string;
  
  // Event handlers
  onClick?: () => void;
}
```

## Styling Guidelines

### Colors
- Primary: Use for main actions
- Secondary: Use for secondary actions
- Error: Use for error states
- Success: Use for success states
- Warning: Use for warnings

### Spacing
- Use 4px, 8px, 12px, 16px, 24px, 32px, 48px, 64px
- Maintain consistent padding and margins

### Typography
- Headings: Use font-weight 600-700
- Body: Use font-weight 400-500
- Small text: Use font-weight 400

## Output Format

When creating components:
1. Create the component file
2. Add TypeScript types
3. Include basic tests
4. Add Storybook stories (if applicable)
5. Document usage examples
