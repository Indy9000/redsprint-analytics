import 'models/event.dart';

enum FilterOperator {
  equals,
  notEquals,
  contains,
  greaterThan,
  lessThan,
  greaterOrEqual,
  lessOrEqual,
}

class Filter {
  final String field;
  final FilterOperator operator;
  final dynamic value;

  Filter(this.field, this.operator, this.value);

  bool matches(AnalyticsEvent event) {
    final fieldValue = event.getProperty(field);

    if (fieldValue == null) return false;

    switch (operator) {
      case FilterOperator.equals:
        return fieldValue == value;
      case FilterOperator.notEquals:
        return fieldValue != value;
      case FilterOperator.contains:
        return fieldValue.toString().contains(value.toString());
      case FilterOperator.greaterThan:
        return _compare(fieldValue, value) > 0;
      case FilterOperator.lessThan:
        return _compare(fieldValue, value) < 0;
      case FilterOperator.greaterOrEqual:
        return _compare(fieldValue, value) >= 0;
      case FilterOperator.lessOrEqual:
        return _compare(fieldValue, value) <= 0;
    }
  }

  int _compare(dynamic a, dynamic b) {
    if (a is Comparable && b is Comparable) {
      return a.compareTo(b);
    }
    return 0;
  }
}

class QueryEngine {
  List<AnalyticsEvent> filter(List<AnalyticsEvent> events, List<Filter> filters) {
    return events.where((event) {
      return filters.every((filter) => filter.matches(event));
    }).toList();
  }

  Map<String, int> count(List<AnalyticsEvent> events) {
    return {'count': events.length};
  }

  Map<String, int> countBy(List<AnalyticsEvent> events, String field) {
    final counts = <String, int>{};

    for (final event in events) {
      final value = event.getProperty(field);
      if (value != null) {
        final key = value.toString();
        counts[key] = (counts[key] ?? 0) + 1;
      }
    }

    return counts;
  }

  Map<String, dynamic> groupBy(
    List<AnalyticsEvent> events,
    String field,
    String aggregateField,
    String aggregateFunction,
  ) {
    final groups = <String, List<dynamic>>{};

    for (final event in events) {
      final groupValue = event.getProperty(field);
      if (groupValue != null) {
        final key = groupValue.toString();
        groups.putIfAbsent(key, () => []);

        final aggValue = event.getProperty(aggregateField);
        if (aggValue != null) {
          groups[key]!.add(aggValue);
        }
      }
    }

    final result = <String, dynamic>{};
    for (final entry in groups.entries) {
      result[entry.key] = _aggregate(entry.value, aggregateFunction);
    }

    return result;
  }

  dynamic _aggregate(List<dynamic> values, String function) {
    if (values.isEmpty) return 0;

    switch (function.toLowerCase()) {
      case 'count':
        return values.length;
      case 'sum':
        return values.fold<num>(0, (sum, val) {
          if (val is num) return sum + val;
          return sum;
        });
      case 'avg':
      case 'average':
        final sum = values.fold<num>(0, (s, val) {
          if (val is num) return s + val;
          return s;
        });
        return sum / values.length;
      case 'min':
        return values
            .where((v) => v is num)
            .fold<num>(double.infinity, (min, val) => val < min ? val : min);
      case 'max':
        return values
            .where((v) => v is num)
            .fold<num>(double.negativeInfinity, (max, val) => val > max ? val : max);
      default:
        return values.length;
    }
  }

  int countUnique(List<AnalyticsEvent> events, String field) {
    final uniqueValues = <String>{};

    for (final event in events) {
      final value = event.getProperty(field);
      if (value != null) {
        uniqueValues.add(value.toString());
      }
    }

    return uniqueValues.length;
  }

  List<MapEntry<String, int>> topN(
    List<AnalyticsEvent> events,
    String field,
    int n,
  ) {
    final counts = countBy(events, field);
    final sorted = counts.entries.toList()
      ..sort((a, b) => b.value.compareTo(a.value));

    return sorted.take(n).toList();
  }

  Map<String, dynamic> analytics(List<AnalyticsEvent> events) {
    return {
      'total_events': events.length,
      'unique_sessions': countUnique(events, 'user.session_id'),
      'unique_users': countUnique(events, 'user.anonymous_id'),
      'apps': countBy(events, 'app_id'),
      'event_types': countBy(events, 'event_type'),
      'event_names': countBy(events, 'event_name'),
      'platforms': countBy(events, 'device.platform'),
      'locales': countBy(events, 'device.locale'),
    };
  }

  Map<String, int> eventsByDate(List<AnalyticsEvent> events) {
    final counts = <String, int>{};

    for (final event in events) {
      final date = event.timestamp.toIso8601String().split('T')[0];
      counts[date] = (counts[date] ?? 0) + 1;
    }

    return Map.fromEntries(
      counts.entries.toList()..sort((a, b) => a.key.compareTo(b.key)),
    );
  }

  Map<String, int> eventsByHour(List<AnalyticsEvent> events) {
    final counts = <String, int>{};

    for (final event in events) {
      final hour = event.timestamp.hour.toString().padLeft(2, '0');
      counts[hour] = (counts[hour] ?? 0) + 1;
    }

    return Map.fromEntries(
      counts.entries.toList()..sort((a, b) => a.key.compareTo(b.key)),
    );
  }

  Map<String, dynamic> analyzeAttribution(
    List<AnalyticsEvent> events,
    Filter conversionFilter,
    Filter sourceFilter,
  ) {
    // Group events by anonymous_id
    final eventsByUser = <String, List<AnalyticsEvent>>{};

    for (final event in events) {
      final anonymousId = event.user?.anonymousId;
      if (anonymousId != null) {
        eventsByUser.putIfAbsent(anonymousId, () => []);
        eventsByUser[anonymousId]!.add(event);
      }
    }

    // Sort events by timestamp for each user
    for (final userEvents in eventsByUser.values) {
      userEvents.sort((a, b) => a.timestamp.compareTo(b.timestamp));
    }

    // Find conversions and check for attribution
    final attributedUsers = <Map<String, dynamic>>[];
    final sourceBreakdown = <String, int>{};
    var totalConversions = 0;
    var attributedConversions = 0;

    for (final entry in eventsByUser.entries) {
      final anonymousId = entry.key;
      final userEvents = entry.value;

      // Find conversion events
      final conversionEvents = userEvents.where((e) => conversionFilter.matches(e)).toList();

      if (conversionEvents.isEmpty) continue;

      totalConversions += conversionEvents.length;

      // For each conversion, look for earlier source events
      for (final conversionEvent in conversionEvents) {
        final earlierEvents = userEvents.where((e) =>
          e.timestamp.isBefore(conversionEvent.timestamp)
        ).toList();

        final sourceEvents = earlierEvents.where((e) => sourceFilter.matches(e)).toList();

        if (sourceEvents.isNotEmpty) {
          attributedConversions++;
          final firstSourceEvent = sourceEvents.first;
          final duration = conversionEvent.timestamp.difference(firstSourceEvent.timestamp);

          // Format duration nicely
          String formatDuration(Duration d) {
            if (d.inDays > 0) {
              return '${d.inDays}d ${d.inHours % 24}h';
            } else if (d.inHours > 0) {
              return '${d.inHours}h ${d.inMinutes % 60}m';
            } else if (d.inMinutes > 0) {
              return '${d.inMinutes}m ${d.inSeconds % 60}s';
            } else {
              return '${d.inSeconds}s';
            }
          }

          attributedUsers.add({
            'anonymous_id': anonymousId,
            'source_timestamp': firstSourceEvent.timestamp.toIso8601String(),
            'conversion_timestamp': conversionEvent.timestamp.toIso8601String(),
            'time_to_conversion': formatDuration(duration),
          });

          // Track source breakdown (e.g., by referrer)
          final sourceValue = firstSourceEvent.getProperty(sourceFilter.field);
          if (sourceValue != null) {
            final key = sourceValue.toString();
            sourceBreakdown[key] = (sourceBreakdown[key] ?? 0) + 1;
          }

          break; // Only count first conversion per user
        }
      }
    }

    final attributionRate = totalConversions > 0
        ? ((attributedConversions / totalConversions) * 100).toStringAsFixed(1)
        : '0.0';

    return {
      'total_conversions': totalConversions,
      'attributed_conversions': attributedConversions,
      'attribution_rate': attributionRate,
      'attributed_users': attributedUsers,
      'source_breakdown': sourceBreakdown,
    };
  }
}
