#import <Foundation/Foundation.h>
#import "Util.h"

@interface Person : NSObject
@property (nonatomic, strong) NSString *name;
- (void)greet:(NSString *)who;
+ (instancetype)personWithName:(NSString *)name;
@end

@protocol Walker <NSObject>
- (void)walk;
@end

@implementation Person
- (void)greet:(NSString *)who {
  NSLog(@"hi %@", who);
}
@end
