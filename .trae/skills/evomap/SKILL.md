---
name: "evomap"
description: "Evolutionary mapping and strategy optimization for continuous improvement. Invoke when user needs adaptive learning, strategy evolution, or performance optimization over time."
---

# EvoMap & Evolver Skill

This skill provides evolutionary algorithm capabilities for continuous learning, strategy optimization, and adaptive behavior improvement. It enables the AI agent to evolve its approaches based on feedback and performance metrics.

## Capabilities

- **Evolutionary Algorithms**: Genetic algorithms and evolutionary strategies
- **Strategy Optimization**: Continuous improvement of decision-making processes
- **Performance Tracking**: Monitor and analyze agent performance over time
- **Adaptive Learning**: Adjust behavior based on feedback and results
- **Knowledge Evolution**: Refine and expand knowledge base dynamically
- **Meta-Learning**: Learn how to learn more effectively

## Usage

### Strategy Evolution
```
/evolve strategy conversation-flow
/evolve strategy code-generation -generations=50
```

### Performance Optimization
```
/optimize response-time
/optimize accuracy -metric=f1-score
```

### Learning Adaptation
```
/adapt learning-rate 0.001
/adapt exploration-rate 0.3
```

### Evolution Options
- `-generations`: Number of evolution generations
- `-population`: Population size for genetic algorithms
- `-metric`: Optimization metric (accuracy, speed, efficiency)
- `-mutation-rate`: Rate of random changes in evolution

## Core Evolutionary Mechanisms

### 1. Genetic Algorithm Framework
- **Selection**: Choose best-performing strategies
- **Crossover**: Combine successful approaches
- **Mutation**: Introduce random variations
- **Evaluation**: Measure strategy performance

### 2. Strategy Representation
- **Behavior Trees**: Represent decision processes
- **Parameter Sets**: Optimize configuration values
- **Rule Systems**: Evolve rule-based approaches
- **Neural Weights**: Optimize model parameters

### 3. Fitness Functions
- **Accuracy**: Task success rates
- **Efficiency**: Resource usage optimization
- **Speed**: Response time improvement
- **User Satisfaction**: Feedback-based scoring

## Examples

1. **Optimize conversation flow**
   ```
   /evolve strategy dialogue -generations=100
   ```

2. **Improve code generation**
   ```
   /optimize code-quality -metric=readability
   ```

3. **Adapt to user preferences**
   ```
   /adapt personalization user-preference-learning
   ```

4. **Evolve search strategies**
   ```
   /evolve search-algorithm web-search-patterns
   ```

## Evolutionary Processes

### Continuous Improvement Cycle
1. **Measure**: Track current performance metrics
2. **Analyze**: Identify areas for improvement
3. **Generate**: Create new strategy variations
4. **Test**: Evaluate new approaches
5. **Select**: Choose best performers
6. **Repeat**: Continuous evolution cycle

### Adaptation Mechanisms
- **Reinforcement Learning**: Reward-based adaptation
- **Transfer Learning**: Apply knowledge across domains
- **Meta-Learning**: Optimize learning processes
- **Multi-objective Optimization**: Balance competing goals

## Integration Points

### With Web Search Skill
- Evolve better search query formulations
- Optimize result filtering and ranking
- Adapt to different information domains

### With Web Crawler Skill
- Improve content extraction patterns
- Optimize crawling strategies
- Adapt to different website structures

### With Other Skills
- Evolve skill combination strategies
- Optimize skill execution order
- Adapt to changing task requirements

## Advanced Features

### 1. Evolutionary Memory
- Store successful strategies
- Recall and reuse proven approaches
- Transfer learning between domains

### 2. Multi-modal Evolution
- Combine different algorithm types
- Hybrid evolutionary approaches
- Ensemble strategy optimization

### 3. Real-time Adaptation
- Dynamic parameter adjustment
- Online learning during operation
- Immediate strategy updates

## Best Practices

- Maintain diversity in evolutionary populations
- Balance exploration vs exploitation
- Preserve successful strategies
- Monitor for convergence and stagnation
- Implement constraint handling

## Performance Metrics

- **Success Rate**: Percentage of successful task completions
- **Efficiency**: Resource usage relative to results
- **Adaptability**: Speed of adaptation to new situations
- **Robustness**: Performance across varied conditions
- **Scalability**: Handling of increasing complexity

## Implementation Notes

This skill requires:
- Persistent storage for strategy archives
- Performance tracking infrastructure
- Evaluation framework for fitness functions
- Version control for evolved strategies
- Backup mechanisms for strategy preservation