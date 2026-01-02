import 'dart:io';
import 'dart:convert';
import 'package:path/path.dart' as path;
import 'models/event.dart';

class EventLoader {
  final String dataPath;

  EventLoader(this.dataPath);

  Future<List<AnalyticsEvent>> loadAllEvents({
    String? appId,
    DateTime? startDate,
    DateTime? endDate,
  }) async {
    final events = <AnalyticsEvent>[];
    final dataDir = Directory(dataPath);

    if (!await dataDir.exists()) {
      throw Exception('Data directory not found: $dataPath');
    }

    await for (final appDir in dataDir.list()) {
      if (appDir is! Directory) continue;

      final currentAppId = path.basename(appDir.path);

      // Skip if filtering by app_id and this is not the target app
      if (appId != null && currentAppId != appId) continue;

      await for (final dateDir in appDir.list()) {
        if (dateDir is! Directory) continue;

        final dateDirName = path.basename(dateDir.path);

        // Skip if filtering by date range
        if (startDate != null || endDate != null) {
          try {
            final dirDate = _parseDate(dateDirName);
            if (startDate != null && dirDate.isBefore(startDate)) continue;
            if (endDate != null && dirDate.isAfter(endDate)) continue;
          } catch (e) {
            continue; // Skip directories that don't match date format
          }
        }

        await for (final file in dateDir.list()) {
          if (file is! File || !file.path.endsWith('.json')) continue;

          try {
            final content = await file.readAsString();
            final json = jsonDecode(content) as Map<String, dynamic>;
            events.add(AnalyticsEvent.fromJson(json));
          } catch (e) {
            print('Warning: Failed to parse ${file.path}: $e');
          }
        }
      }
    }

    return events;
  }

  Future<List<String>> getAppIds() async {
    final dataDir = Directory(dataPath);
    final appIds = <String>[];

    if (!await dataDir.exists()) {
      throw Exception('Data directory not found: $dataPath');
    }

    await for (final entity in dataDir.list()) {
      if (entity is Directory) {
        appIds.add(path.basename(entity.path));
      }
    }

    return appIds;
  }

  Future<Map<String, List<String>>> getDateRangeByApp() async {
    final dataDir = Directory(dataPath);
    final result = <String, List<String>>{};

    if (!await dataDir.exists()) {
      throw Exception('Data directory not found: $dataPath');
    }

    await for (final appDir in dataDir.list()) {
      if (appDir is! Directory) continue;

      final appId = path.basename(appDir.path);
      final dates = <String>[];

      await for (final dateDir in appDir.list()) {
        if (dateDir is Directory) {
          dates.add(path.basename(dateDir.path));
        }
      }

      dates.sort();
      result[appId] = dates;
    }

    return result;
  }

  DateTime _parseDate(String dateStr) {
    // Format: YYYYMMDD
    final year = int.parse(dateStr.substring(0, 4));
    final month = int.parse(dateStr.substring(4, 6));
    final day = int.parse(dateStr.substring(6, 8));
    return DateTime(year, month, day);
  }
}
