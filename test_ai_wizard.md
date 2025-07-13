# AI Configuration Wizard Test Scenarios

## Test 1: Ollama Setup
```
/ai-config
2
(enter default URL)
1
```

## Test 2: LM Studio Setup  
```
/ai-config
3
http://localhost:1234
1
```

## Test 3: OpenRouter Setup (with API key)
```
/ai-config
1
sk-or-v1-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
1
```

## Test 4: Help and Status
```
/ai-config status
/ai-config help
```

## Expected Flow:
1. Provider selection with clear descriptions
2. API key prompt for OpenRouter (but not local providers)
3. Base URL configuration for local providers
4. Live model fetching from the selected provider's API
5. Model selection with pricing information (for OpenRouter)
6. Final configuration summary
7. Prompt update with new AI info