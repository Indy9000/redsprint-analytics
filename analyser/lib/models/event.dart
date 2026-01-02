class AnalyticsEvent {
  final String eventId;
  final DateTime timestamp;
  final String appId;
  final String eventType;
  final String eventName;
  final User? user;
  final Device? device;
  final Location? location;
  final WebSpecific? webSpecific;
  final Map<String, dynamic> properties;

  AnalyticsEvent({
    required this.eventId,
    required this.timestamp,
    required this.appId,
    required this.eventType,
    required this.eventName,
    this.user,
    this.device,
    this.location,
    this.webSpecific,
    required this.properties,
  });

  factory AnalyticsEvent.fromJson(Map<String, dynamic> json) {
    return AnalyticsEvent(
      eventId: json['event_id'] as String,
      timestamp: DateTime.parse(json['timestamp'] as String),
      appId: json['app_id'] as String,
      eventType: json['event_type'] as String,
      eventName: json['event_name'] as String,
      user: json['user'] != null ? User.fromJson(json['user']) : null,
      device: json['device'] != null ? Device.fromJson(json['device']) : null,
      location: json['location'] != null ? Location.fromJson(json['location']) : null,
      webSpecific: json['web_specific'] != null
          ? WebSpecific.fromJson(json['web_specific'])
          : null,
      properties: json['properties'] as Map<String, dynamic>? ?? {},
    );
  }

  dynamic getProperty(String path) {
    final parts = path.split('.');

    if (parts[0] == 'event_id') return eventId;
    if (parts[0] == 'timestamp') return timestamp;
    if (parts[0] == 'app_id') return appId;
    if (parts[0] == 'event_type') return eventType;
    if (parts[0] == 'event_name') return eventName;

    if (parts[0] == 'user' && user != null) {
      if (parts.length == 1) return user;
      if (parts[1] == 'session_id') return user!.sessionId;
      if (parts[1] == 'anonymous_id') return user!.anonymousId;
    }

    if (parts[0] == 'device' && device != null) {
      if (parts.length == 1) return device;
      if (parts[1] == 'platform') return device!.platform;
      if (parts[1] == 'os_version') return device!.osVersion;
      if (parts[1] == 'device_model') return device!.deviceModel;
      if (parts[1] == 'screen_resolution') return device!.screenResolution;
      if (parts[1] == 'locale') return device!.locale;
      if (parts[1] == 'timezone') return device!.timezone;
    }

    if (parts[0] == 'location' && location != null) {
      if (parts.length == 1) return location;
      if (parts[1] == 'ip') return location!.ip;
    }

    if (parts[0] == 'web_specific' && webSpecific != null) {
      if (parts.length == 1) return webSpecific;
      if (parts[1] == 'user_agent') return webSpecific!.userAgent;
      if (parts[1] == 'referrer') return webSpecific!.referrer;
      if (parts[1] == 'page_url') return webSpecific!.pageUrl;
      if (parts[1] == 'page_title') return webSpecific!.pageTitle;
      if (parts[1] == 'utm_source') return webSpecific!.utmSource;
      if (parts[1] == 'utm_medium') return webSpecific!.utmMedium;
      if (parts[1] == 'utm_campaign') return webSpecific!.utmCampaign;
      if (parts[1] == 'utm_content') return webSpecific!.utmContent;
      if (parts[1] == 'utm_term') return webSpecific!.utmTerm;
      if (parts[1] == 'utm_id') return webSpecific!.utmId;
    }

    if (parts[0] == 'properties') {
      if (parts.length == 1) return properties;
      return properties[parts[1]];
    }

    return null;
  }
}

class User {
  final String? sessionId;
  final String? anonymousId;

  User({this.sessionId, this.anonymousId});

  factory User.fromJson(Map<String, dynamic> json) {
    return User(
      sessionId: json['session_id'] as String?,
      anonymousId: json['anonymous_id'] as String?,
    );
  }
}

class Device {
  final String? platform;
  final String? osVersion;
  final String? deviceModel;
  final String? screenResolution;
  final String? locale;
  final String? timezone;

  Device({
    this.platform,
    this.osVersion,
    this.deviceModel,
    this.screenResolution,
    this.locale,
    this.timezone,
  });

  factory Device.fromJson(Map<String, dynamic> json) {
    return Device(
      platform: json['platform'] as String?,
      osVersion: json['os_version'] as String?,
      deviceModel: json['device_model'] as String?,
      screenResolution: json['screen_resolution'] as String?,
      locale: json['locale'] as String?,
      timezone: json['timezone'] as String?,
    );
  }
}

class Location {
  final String? ip;

  Location({this.ip});

  factory Location.fromJson(Map<String, dynamic> json) {
    return Location(
      ip: json['ip'] as String?,
    );
  }
}

class WebSpecific {
  final String? userAgent;
  final String? referrer;
  final String? pageUrl;
  final String? pageTitle;
  final String? utmSource;
  final String? utmMedium;
  final String? utmCampaign;
  final String? utmContent;
  final String? utmTerm;
  final String? utmId;

  WebSpecific({
    this.userAgent,
    this.referrer,
    this.pageUrl,
    this.pageTitle,
    this.utmSource,
    this.utmMedium,
    this.utmCampaign,
    this.utmContent,
    this.utmTerm,
    this.utmId,
  });

  factory WebSpecific.fromJson(Map<String, dynamic> json) {
    return WebSpecific(
      userAgent: json['user_agent'] as String?,
      referrer: json['referrer'] as String?,
      pageUrl: json['page_url'] as String?,
      pageTitle: json['page_title'] as String?,
      utmSource: json['utm_source'] as String?,
      utmMedium: json['utm_medium'] as String?,
      utmCampaign: json['utm_campaign'] as String?,
      utmContent: json['utm_content'] as String?,
      utmTerm: json['utm_term'] as String?,
      utmId: json['utm_id'] as String?,
    );
  }
}
